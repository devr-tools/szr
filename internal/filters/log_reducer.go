package filters

import (
	"fmt"
	"strings"
)

func ReduceLogLines(lines []string, maxLines int) []string {
	entries, total := collectLogLineEntries(lines)
	if len(entries) == 0 {
		return nil
	}
	maxLines = normalizeReducedLogLimit(maxLines)
	if len(entries) <= maxLines {
		return renderReducedLogLines(entries, total, len(entries))
	}
	return renderReducedLogLines(selectReducedLogEntries(entries, maxLines), total, maxLines)
}

func normalizeReducedLogLimit(maxLines int) int {
	if maxLines <= 0 {
		return 20
	}
	return maxLines
}

func selectReducedLogEntries(entries []logLineEntry, maxLines int) []logLineEntry {
	selected := make(map[string]struct{}, maxLines)
	for _, kind := range []logLineKind{
		logLineKindError,
		logLineKindAnchor,
		logLineKindHint,
		logLineKindDefault,
		logLineKindBoilerplate,
	} {
		selectReducedLogKind(entries, selected, maxLines, kind)
	}
	return orderedReducedLogEntries(entries, selected)
}

func selectReducedLogKind(entries []logLineEntry, selected map[string]struct{}, maxLines int, kind logLineKind) {
	for _, entry := range entries {
		if len(selected) >= maxLines {
			return
		}
		if entry.kind == kind {
			selected[entry.key] = struct{}{}
		}
	}
}

func renderReducedLogLines(entries []logLineEntry, total, maxLines int) []string {
	rendered := make([]string, 0, minInt(len(entries), maxLines)+1)
	keptRaw := 0
	for _, entry := range entries {
		rendered = append(rendered, formatReducedLogLine(entry))
		keptRaw += entry.count
	}
	if omitted := total - keptRaw; omitted > 0 {
		rendered = append(rendered, fmt.Sprintf("... +%d more log lines", omitted))
	}
	return rendered
}

func formatReducedLogLine(entry logLineEntry) string {
	if entry.count <= 1 {
		return entry.line
	}
	return fmt.Sprintf("%s (x%d)", entry.line, entry.count)
}

// Log-content sniffing: content is treated as log-shaped when the majority of
// the first logSniffWindow non-empty lines start with a timestamp or a
// log-level token.
const (
	logSniffWindow   = 40
	logSniffMinLines = 5
)

// LooksLikeLogText reports whether the input looks like a service log:
// a majority of its first lines start with a timestamp or log-level token.
func LooksLikeLogText(input string) bool {
	lines := nonEmptyLines(StripANSI(input))
	if len(lines) > logSniffWindow {
		lines = lines[:logSniffWindow]
	}
	return looksLikeLogLines(lines)
}

func looksLikeLogLines(lines []string) bool {
	if len(lines) > logSniffWindow {
		lines = lines[:logSniffWindow]
	}
	if len(lines) < logSniffMinLines {
		return false
	}
	shaped := 0
	for _, line := range lines {
		if isLogShapedLine(line) {
			shaped++
		}
	}
	return shaped*2 > len(lines)
}

// SummarizeLogText renders log content as a severity histogram followed by
// deduplicated messages with counts and first–last clock ranges. Every
// distinct ERROR/FATAL message is kept regardless of maxLines; WARN and then
// lower severities fill the remaining budget by count. It returns the
// rendered text plus the total number of non-empty input lines.
func SummarizeLogText(input string, maxLines int) (string, int) {
	agg := newLogAggregate()
	for _, line := range nonEmptyLines(StripANSI(input)) {
		agg.ingest(line)
	}
	return strings.Join(agg.render(maxLines), "\n"), agg.totalLines
}

// LogAwareReadReducer is the streaming reducer behind cat-read for
// log-candidate files. It buffers only a small sniff window; once the content
// is recognized as log-shaped it switches to an incremental bounded-memory
// aggregate (dedup counts, no full buffering), otherwise it falls back to
// buffering the whole stream for the provided render callback.
type LogAwareReadReducer struct {
	maxLines         int
	fallbackRender   func(string) string
	fallbackRecovery func(string) (string, string, bool)
	scanner          lineScanner
	bytesParsed      int
	decided          bool
	isLog            bool
	finished         bool
	sniff            []string
	raw              textBuffer
	agg              *logAggregate
}

func NewLogAwareReadReducer(
	maxLines int,
	fallbackRender func(string) string,
	fallbackRecovery func(string) (string, string, bool),
) *LogAwareReadReducer {
	return &LogAwareReadReducer{
		maxLines:         maxLines,
		fallbackRender:   fallbackRender,
		fallbackRecovery: fallbackRecovery,
		agg:              newLogAggregate(),
	}
}

func (r *LogAwareReadReducer) ConsumeStdout(chunk []byte) {
	r.bytesParsed += len(chunk)
	switch {
	case r.decided && r.isLog:
		r.scanner.Consume(chunk, r.agg.ingest)
	case r.decided:
		r.raw.Consume(chunk)
	default:
		r.raw.Consume(chunk)
		r.scanner.Consume(chunk, r.ingestSniffLine)
		if len(r.sniff) >= logSniffWindow {
			r.decide()
		}
	}
}

// ConsumeStderr ignores stderr; cat-read is a stdout-only profile.
func (r *LogAwareReadReducer) ConsumeStderr([]byte) {}

func (r *LogAwareReadReducer) Result() string {
	r.finalize()
	if r.isLog {
		return strings.Join(r.agg.render(r.maxLines), "\n")
	}
	return r.fallbackRender(strings.TrimSpace(r.raw.String()))
}

func (r *LogAwareReadReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *LogAwareReadReducer) FallbackUsed() bool {
	return false
}

func (r *LogAwareReadReducer) RecoveryInfo() (string, string, bool) {
	r.finalize()
	if !r.isLog {
		if r.fallbackRecovery == nil {
			return NoRecovery()
		}
		return r.fallbackRecovery(strings.TrimSpace(r.raw.String()))
	}
	if r.agg.totalLines <= len(r.agg.render(r.maxLines)) {
		return NoRecovery()
	}
	return FullOutputRecovery(fmt.Sprintf(
		"summarized %d log lines (%d distinct messages)",
		r.agg.totalLines, r.agg.distinctTotal(),
	))
}

func (r *LogAwareReadReducer) ingestSniffLine(line string) {
	r.sniff = append(r.sniff, line)
}

// decide classifies the buffered sniff window. Log-shaped content replays the
// sniffed lines into the aggregate and drops the raw buffer; anything else
// keeps buffering raw text for the fallback renderer.
func (r *LogAwareReadReducer) decide() {
	r.decided = true
	r.isLog = looksLikeLogLines(r.sniff)
	if r.isLog {
		for _, line := range r.sniff {
			r.agg.ingest(line)
		}
		r.raw = textBuffer{}
	}
	r.sniff = nil
}

func (r *LogAwareReadReducer) finalize() {
	if r.finished {
		return
	}
	r.finished = true
	if !r.decided {
		r.scanner.Finish(r.ingestSniffLine)
		r.decide()
		return
	}
	if r.isLog {
		r.scanner.Finish(r.agg.ingest)
	}
}
