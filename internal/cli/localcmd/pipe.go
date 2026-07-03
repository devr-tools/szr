package localcmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/jsonquery"
	"github.com/devr-tools/szr/internal/filters/patch"
)

// szr pipe summarizes output that is already flowing through a shell pipe
// (`<cmd> | szr pipe`). Unlike `szr run` there is no child process on szr's
// side: the read end of a pipe cannot observe the producer's exit status, so
// the summary describes stream content only and must never claim the
// producer succeeded or failed. RunPipe's own exit code covers only its own
// work: 0 when filtering succeeded, 1 for stdin/tee I/O failures, 2 for
// usage errors (including a terminal-attached stdin).
const (
	// pipeHeadCapBytes bounds the buffered head for non-streaming routes;
	// beyond it only a bounded tail is retained and the render carries an
	// explicit truncation note.
	pipeHeadCapBytes = 8 << 20
	// pipeTailCapBytes is the trailing-byte ring kept once the head cap is
	// exceeded, so the end of a truncated stream stays inspectable.
	pipeTailCapBytes = 64 << 10
	// pipeSniffBytes is how much leading input the auto router inspects
	// before committing to a reducer.
	pipeSniffBytes = 64 << 10
	pipeReadChunk  = 32 << 10
	pipeTailLines  = 3
)

const (
	pipeRouteAuto   = "auto"
	pipeRouteLog    = "log"
	pipeRouteJSON   = "json"
	pipeRouteTest   = "test"
	pipeRouteDiff   = "diff"
	pipeRouteBinary = "binary"
	pipeRouteText   = "text"
)

const pipeHelpText = `usage: <cmd> | szr pipe [--hint auto|log|json|test|diff] [--max-lines N] [--tee <path>]

Summarize output that is already flowing through a pipe. Content is sniffed
and routed to the matching reducer: service logs become a severity summary,
JSON and NDJSON become structural previews (uniform object arrays render as
one table), binary bytes become a printable-strings digest, and anything
else goes through the failure-signal line reducer.

flags:
  --hint <kind>   force a reducer instead of sniffing:
                    log   severity histogram + deduplicated messages
                    json  JSON/NDJSON structural preview
                    test  go test -json stream or plain test output
                    diff  unified diff / patch text
                    auto  content sniffing (default)
  --max-lines N   override the configured preview line budget
  --tee <path>    also write the raw, unfiltered stream to <path>

honesty: the read side of a pipe cannot see the producing command's exit
status. The summary describes stream content only; it never states that the
producer succeeded or failed. szr pipe exits 0 when filtering succeeds and
nonzero only for its own errors (usage errors, stdin/tee I/O failures).

memory: log-shaped input streams through a bounded aggregate. Other input
is buffered up to 8MB; beyond that the summary covers the buffered head
plus a retained tail and says so explicitly. Use --tee to keep the full
stream on disk.
`

type pipeOptions struct {
	hint     string
	teePath  string
	maxLines int
	help     bool
}

// pipeCapture is the bounded view of the consumed stream. Exactly one of
// logStream (streaming log route) or head/tail (buffered routes) carries the
// content; total always counts every byte read.
type pipeCapture struct {
	route     string
	head      []byte
	tail      []byte
	total     int
	truncated bool
	logStream *filters.StreamingLogSummarizer
}

// RunPipe implements `szr pipe`. stdin and stdinIsTerminal are injected so
// tests can drive the command without touching os.Stdin.
func RunPipe(rt Runtime, cfg config.Config, args []string, stdin io.Reader, stdinIsTerminal bool) int {
	opts, err := parsePipeOptions(args, adjustCountForReasoningMode(cfg.ReasoningBudgetMode, cfg.MaxPreviewLines))
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: pipe: %v\n", err)
		fmt.Fprintln(rt.Stderr, "usage: <cmd> | szr pipe [--hint auto|log|json|test|diff] [--max-lines N] [--tee <path>]")
		return 2
	}
	if opts.help {
		fmt.Fprint(rt.Stdout, pipeHelpText)
		return 0
	}
	if stdinIsTerminal {
		fmt.Fprintln(rt.Stderr, "szr: pipe reads stdin from a pipe; run it as `<cmd> | szr pipe` (see `szr pipe --help`)")
		return 2
	}
	return runPipeStream(rt, opts, stdin)
}

func parsePipeOptions(args []string, defaultMaxLines int) (pipeOptions, error) {
	opts := pipeOptions{hint: pipeRouteAuto, maxLines: defaultMaxLines}
	for i := 0; i < len(args); i++ {
		if args[i] == "-h" || args[i] == "--help" {
			opts.help = true
			continue
		}
		if err := parsePipeFlag(&opts, args, &i); err != nil {
			return opts, err
		}
	}
	if !validPipeHint(opts.hint) {
		return opts, fmt.Errorf("unknown --hint %q (expected auto, log, json, test, or diff)", opts.hint)
	}
	return opts, nil
}

func parsePipeFlag(opts *pipeOptions, args []string, i *int) error {
	flag := args[*i]
	if flag != "--hint" && flag != "--tee" && flag != "--max-lines" {
		return fmt.Errorf("unknown argument %q", flag)
	}
	value, err := pipeFlagValue(args, i)
	if err != nil {
		return err
	}
	return applyPipeFlag(opts, flag, value)
}

func applyPipeFlag(opts *pipeOptions, flag, value string) error {
	switch flag {
	case "--hint":
		opts.hint = value
	case "--tee":
		opts.teePath = value
	case "--max-lines":
		return setPipeMaxLines(opts, value)
	}
	return nil
}

func setPipeMaxLines(opts *pipeOptions, value string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fmt.Errorf("invalid --max-lines value %q", value)
	}
	opts.maxLines = parsed
	return nil
}

func pipeFlagValue(args []string, i *int) (string, error) {
	flag := args[*i]
	*i++
	if *i >= len(args) {
		return "", fmt.Errorf("missing value for %s", flag)
	}
	return args[*i], nil
}

func validPipeHint(hint string) bool {
	switch hint {
	case pipeRouteAuto, pipeRouteLog, pipeRouteJSON, pipeRouteTest, pipeRouteDiff:
		return true
	}
	return false
}

func runPipeStream(rt Runtime, opts pipeOptions, stdin io.Reader) int {
	capture, code := consumePipeWithTee(rt, opts, stdin)
	if code != 0 {
		return code
	}
	writePipeRender(rt, opts, capture)
	return 0
}

// consumePipeWithTee consumes stdin, mirroring the raw stream to the opt-in
// tee file when requested. A nonzero second return value is the exit code
// for szr's own I/O failures.
func consumePipeWithTee(rt Runtime, opts pipeOptions, stdin io.Reader) (pipeCapture, int) {
	var tee *os.File
	if opts.teePath != "" {
		file, err := os.Create(opts.teePath)
		if err != nil {
			fmt.Fprintf(rt.Stderr, "szr: pipe: tee: %v\n", err)
			return pipeCapture{}, 1
		}
		tee = file
		stdin = io.TeeReader(stdin, tee)
	}
	capture, readErr := consumePipe(stdin, opts.hint)
	teeErr := closePipeTee(tee)
	if readErr != nil {
		fmt.Fprintf(rt.Stderr, "szr: pipe: read stdin: %v\n", readErr)
		return capture, 1
	}
	if teeErr != nil {
		fmt.Fprintf(rt.Stderr, "szr: pipe: tee: %v\n", teeErr)
		return capture, 1
	}
	return capture, 0
}

func writePipeRender(rt Runtime, opts pipeOptions, capture pipeCapture) {
	if capture.total == 0 {
		fmt.Fprintln(rt.Stdout, "szr pipe: empty input (0 bytes on stdin)")
		return
	}
	fmt.Fprintln(rt.Stdout, pipeSummaryOrPlaceholder(renderPipeSummary(capture, opts.maxLines), capture.total))
	for _, note := range pipeNotes(opts, capture) {
		fmt.Fprintln(rt.Stdout, note)
	}
}

func closePipeTee(tee *os.File) error {
	if tee == nil {
		return nil
	}
	return tee.Close()
}

// consumePipe reads all of stdin with bounded memory: a sniff window decides
// the route, then log-shaped content streams through the bounded log
// aggregate while everything else buffers up to pipeHeadCapBytes plus a
// bounded tail.
func consumePipe(stdin io.Reader, hint string) (pipeCapture, error) {
	head, eof, err := readPipeSniff(stdin)
	capture := pipeCapture{head: head, total: len(head)}
	if err != nil {
		return capture, err
	}
	capture.route = resolvePipeRoute(hint, head)
	if capture.route == pipeRouteLog {
		return consumePipeLog(stdin, capture, eof)
	}
	return consumePipeBuffered(stdin, capture, eof)
}

// readPipeSniff reads the leading routing window. It returns the bytes read,
// whether the stream already ended, and any read error.
func readPipeSniff(stdin io.Reader) ([]byte, bool, error) {
	head := make([]byte, 0, pipeSniffBytes)
	buf := make([]byte, pipeReadChunk)
	for len(head) < pipeSniffBytes {
		n, err := stdin.Read(buf)
		head = append(head, buf[:n]...)
		if errors.Is(err, io.EOF) {
			return head, true, nil
		}
		if err != nil {
			return head, false, err
		}
	}
	return head, false, nil
}

// resolvePipeRoute picks a reducer. A non-auto hint always wins; otherwise
// the sniff window is classified by content alone, since a pipe carries no
// command context: binary bytes, then log-shaped lines, then a leading '{'
// or '[' (validated as JSON by the renderer, which falls back to compact
// lines when it does not parse), then generic text.
func resolvePipeRoute(hint string, sniff []byte) string {
	if hint != "" && hint != pipeRouteAuto {
		return hint
	}
	switch {
	case len(sniff) == 0:
		return pipeRouteText
	case filters.IsBinaryish(sniff):
		return pipeRouteBinary
	case filters.LooksLikeLogText(string(sniff)):
		return pipeRouteLog
	case pipeLooksLikeJSON(sniff):
		return pipeRouteJSON
	default:
		return pipeRouteText
	}
}

func pipeLooksLikeJSON(sniff []byte) bool {
	trimmed := bytes.TrimLeft(sniff, " \t\r\n")
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

// consumePipeLog streams the remainder of stdin through the bounded log
// aggregate; the raw bytes are never buffered, so memory stays fixed no
// matter how large the stream is.
func consumePipeLog(stdin io.Reader, capture pipeCapture, eof bool) (pipeCapture, error) {
	capture.logStream = filters.NewStreamingLogSummarizer()
	capture.logStream.Consume(capture.head)
	capture.head = nil
	if eof {
		return capture, nil
	}
	buf := make([]byte, pipeReadChunk)
	for {
		n, err := stdin.Read(buf)
		capture.total += n
		capture.logStream.Consume(buf[:n])
		if errors.Is(err, io.EOF) {
			return capture, nil
		}
		if err != nil {
			return capture, err
		}
	}
}

// consumePipeBuffered buffers the stream head up to pipeHeadCapBytes; beyond
// the cap it keeps only a pipeTailCapBytes tail ring and marks the capture
// truncated.
func consumePipeBuffered(stdin io.Reader, capture pipeCapture, eof bool) (pipeCapture, error) {
	if eof {
		return capture, nil
	}
	buf := make([]byte, pipeReadChunk)
	for {
		n, err := stdin.Read(buf)
		capture.total += n
		capture.appendBuffered(buf[:n])
		if errors.Is(err, io.EOF) {
			return capture, nil
		}
		if err != nil {
			return capture, err
		}
	}
}

func (c *pipeCapture) appendBuffered(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	if room := pipeHeadCapBytes - len(c.head); room > 0 {
		take := len(chunk)
		if take > room {
			take = room
		}
		c.head = append(c.head, chunk[:take]...)
		chunk = chunk[take:]
	}
	if len(chunk) == 0 {
		return
	}
	c.truncated = true
	c.tail = append(c.tail, chunk...)
	if overflow := len(c.tail) - pipeTailCapBytes; overflow > 0 {
		c.tail = append(c.tail[:0], c.tail[overflow:]...)
	}
}

func renderPipeSummary(capture pipeCapture, maxLines int) string {
	switch capture.route {
	case pipeRouteLog:
		return capture.logStream.Result(maxLines)
	case pipeRouteJSON:
		return jsonquery.SummarizeQueryOutput(string(capture.head), "", maxLines)
	case pipeRouteDiff:
		return patch.SummarizePatchDiff(string(capture.head), maxLines)
	case pipeRouteTest:
		return renderPipeTestSummary(string(capture.head), maxLines)
	case pipeRouteBinary:
		return filters.SummarizeBinaryish(capture.head, maxLines)
	default:
		return filters.SummarizeGenericFailure(string(capture.head), maxLines)
	}
}

// renderPipeTestSummary handles the test hint: a `go test -json` stream gets
// the structured package/failure summary, anything else goes through the
// failure-signal reducer that `szr test` biasing uses for plain output.
func renderPipeTestSummary(head string, maxLines int) string {
	if strings.HasPrefix(strings.TrimSpace(head), "{") {
		return filters.SummarizeGoTestJSON(head)
	}
	return filters.SummarizeGenericFailure(head, maxLines)
}

// pipeSummaryOrPlaceholder keeps the render honest for degenerate input: the
// generic reducer renders whitespace-only content as "ok", which a reader
// could mistake for producer success that szr cannot actually observe.
func pipeSummaryOrPlaceholder(summary string, total int) string {
	if s := strings.TrimSpace(summary); s != "" && s != "ok" {
		return summary
	}
	return fmt.Sprintf("(no summarizable text in %d bytes of input)", total)
}

func pipeNotes(opts pipeOptions, capture pipeCapture) []string {
	notes := []string{}
	if capture.truncated {
		notes = append(notes, fmt.Sprintf(
			"note: input exceeded the %dMB pipe buffer; the summary covers the first %dMB of %d total bytes",
			pipeHeadCapBytes>>20, pipeHeadCapBytes>>20, capture.total))
		notes = append(notes, pipeTailExcerpt(capture.tail)...)
		if opts.teePath == "" {
			notes = append(notes, "note: re-run with `... | szr pipe --tee <path>` to preserve the full stream")
		}
	}
	if opts.teePath != "" {
		notes = append(notes, fmt.Sprintf("full stream: %s (%d bytes)", opts.teePath, capture.total))
	}
	return notes
}

// pipeTailExcerpt renders the last few retained lines of a truncated stream.
// The first retained line is dropped when others follow, because the tail
// ring usually cuts it mid-line.
func pipeTailExcerpt(tail []byte) []string {
	lines := filters.NonEmptyLines(filters.StripANSI(string(tail)))
	if len(lines) == 0 {
		return nil
	}
	if len(lines) > 1 {
		lines = lines[1:]
	}
	if len(lines) > pipeTailLines {
		lines = lines[len(lines)-pipeTailLines:]
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, fmt.Sprintf("tail (last %d retained lines):", len(lines)))
	for _, line := range lines {
		out = append(out, filters.Clip(line, 160))
	}
	return out
}
