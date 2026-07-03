package filters

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// binaryishSampleBytes bounds the detection scan; binary content shows
	// its control bytes immediately, so a leading sample is enough.
	binaryishSampleBytes = 4096
	// binaryishThresholdPercent is the share of non-printable, non-UTF-8
	// bytes in the sample above which content is treated as binary.
	binaryishThresholdPercent = 10
	// binaryishMinRunChars is the minimum printable-run length worth
	// surfacing; shorter runs are overwhelmingly coincidental byte noise.
	binaryishMinRunChars = 8
)

// IsBinaryish reports whether data looks like binary output rather than
// text: more than 10% of the sampled leading bytes are non-printable bytes
// that are not part of a valid printable UTF-8 sequence or common
// whitespace. Valid multi-byte text (CJK logs, emoji) never trips this.
func IsBinaryish(data []byte) bool {
	sample := data
	if len(sample) > binaryishSampleBytes {
		sample = sample[:binaryishSampleBytes]
	}
	if len(sample) == 0 {
		return false
	}
	binaryBytes := 0
	for i := 0; i < len(sample); {
		r, size := utf8.DecodeRune(sample[i:])
		if r == utf8.RuneError && size == 1 {
			binaryBytes++
			i++
			continue
		}
		if !isBinaryishTextRune(r) {
			binaryBytes += size
		}
		i += size
	}
	return binaryBytes*100 > len(sample)*binaryishThresholdPercent
}

// SummarizeBinaryish renders binary-ish output as a byte-count headline plus
// up to maxLines-1 printable string runs (at least 8 characters each,
// deduplicated, in stream order) so embedded text needles stay discoverable
// without replaying control-byte noise.
func SummarizeBinaryish(data []byte, maxLines int) string {
	text, _ := summarizeBinaryishResult(data, maxLines)
	return text
}

// BinaryishRecoveryInfo reports the full-output recovery plan for a
// binary-ish render: the control-byte payload is always elided, so the raw
// capture is the only complete copy.
func BinaryishRecoveryInfo(data []byte, maxLines int) (string, string, bool) {
	_, omitted := summarizeBinaryishResult(data, maxLines)
	if omitted > 0 {
		return FullOutputRecovery(fmt.Sprintf("omitted %d additional strings and all binary content", omitted))
	}
	return FullOutputRecovery("omitted binary content")
}

func summarizeBinaryishResult(data []byte, maxLines int) (string, int) {
	if maxLines <= 0 {
		maxLines = 12
	}
	runs := extractPrintableRuns(data)
	out := make([]string, 0, maxLines)
	out = append(out, fmt.Sprintf("binary output: %d bytes", len(data)))
	shown := minFilterInt(len(runs), maxLines-1)
	for _, run := range runs[:shown] {
		out = append(out, Clip(run, 160))
	}
	return strings.Join(out, "\n"), len(runs) - shown
}

// extractPrintableRuns collects the deduplicated printable character runs of
// at least binaryishMinRunChars characters, in stream order.
func extractPrintableRuns(data []byte) []string {
	collector := printableRunCollector{seen: map[string]struct{}{}}
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 || !isBinaryishRunRune(r) {
			collector.flush()
			i += size
			continue
		}
		collector.extend(r)
		i += size
	}
	collector.flush()
	return collector.runs
}

// printableRunCollector accumulates printable rune runs and keeps the
// deduplicated ones that reach the minimum length.
type printableRunCollector struct {
	runs    []string
	seen    map[string]struct{}
	current strings.Builder
	chars   int
}

func (c *printableRunCollector) extend(r rune) {
	c.current.WriteRune(r)
	c.chars++
}

func (c *printableRunCollector) flush() {
	if c.chars >= binaryishMinRunChars {
		run := strings.TrimSpace(c.current.String())
		if _, dup := c.seen[run]; run != "" && !dup {
			c.seen[run] = struct{}{}
			c.runs = append(c.runs, run)
		}
	}
	c.current.Reset()
	c.chars = 0
}

// isBinaryishTextRune reports whether a rune is ordinary text for the
// binary-content detector: printable or common whitespace.
func isBinaryishTextRune(r rune) bool {
	return unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t'
}

// isBinaryishRunRune reports whether a rune may extend a printable string
// run. Newlines break runs (they delimit rendered lines); spaces and tabs
// keep multi-word strings such as embedded messages together.
func isBinaryishRunRune(r rune) bool {
	return unicode.IsPrint(r) || r == '\t'
}
