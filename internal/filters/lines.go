package filters

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/filters/declarative"
)

func CompactLines(input string, maxLines int) string {
	result, err := declarative.ApplyBuiltin("compact_lines", StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err == nil {
		return result.Text
	}

	reducer := NewCompactLineReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func InterestingErrorLines(input string, maxLines int) string {
	result, err := declarative.ApplyBuiltin("interesting_error_lines", StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err == nil {
		return result.Text
	}
	return SummarizeGenericFailure(input, maxLines)
}

func DedupeLines(input string, maxLines int) string {
	return strings.Join(ReduceLogLines(nonEmptyLines(StripANSI(input)), maxLines), "\n")
}

func ScannerDedupe(data []byte) string {
	return DedupeLines(string(data), 20)
}

func StripANSI(input string) string {
	var out strings.Builder
	var stripper ansiStripper
	stripper.Consume([]byte(input), func(b byte) {
		out.WriteByte(b)
	})
	return out.String()
}

func nonEmptyLines(input string) []string {
	scanner := lineScanner{}
	lines := []string{}
	scanner.Consume([]byte(input), func(line string) {
		lines = append(lines, line)
	})
	scanner.Finish(func(line string) {
		lines = append(lines, line)
	})
	return lines
}

func NonEmptyLines(input string) []string {
	return nonEmptyLines(input)
}

func clip(input string, max int) string {
	return Clip(input, max)
}

func Clip(input string, max int) string {
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "..."
}

func uniqueStrings(values []string) []string {
	return UniqueStrings(values)
}

func UniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func joinLimitedLines(lines []string, maxLines int) string {
	if len(lines) == 0 {
		return "ok"
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	selected := append([]string{}, lines[:maxLines]...)
	selected = append(selected, fmt.Sprintf("... +%d more lines", len(lines)-maxLines))
	return strings.Join(selected, "\n")
}

func JoinLimitedLines(lines []string, maxLines int) string {
	return joinLimitedLines(lines, maxLines)
}

func FoldConsecutiveLines(lines []string) []string {
	return declarative.FoldConsecutive(lines)
}

func FoldConsecutiveSimilarLines(lines []string) []string {
	return declarative.FoldConsecutiveSimilar(lines)
}

func similarLineKey(line string) string {
	return declarative.SimilarLineKey(line)
}

func SelectUniqueAnchoredLines(lines []string, maxFrames int) []string {
	if maxFrames <= 0 {
		return nil
	}
	out := []string{}
	seen := map[string]struct{}{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		key := DiagnosticAnchor(trimmed)
		if key == "" {
			key = trimmed
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
		if len(out) >= maxFrames {
			break
		}
	}
	return out
}

// diagnosticAnchorExtensions lists the source-file extensions recognized as
// diagnostic anchors ("path/file.ext:line" references). This is the single
// canonical list shared by DiagnosticAnchor and the failure reducer.
var diagnosticAnchorExtensions = []string{
	".go:", ".py:", ".rs:", ".ts:", ".tsx:", ".mts:", ".cts:", ".js:", ".jsx:", ".mjs:", ".cjs:",
	".php:", ".phtml:", ".java:", ".c:", ".cc:", ".cpp:", ".h:", ".hpp:",
	".rb:", ".kt:", ".kts:", ".swift:", ".scala:", ".cs:", ".ex:", ".exs:", ".erl:", ".m:", ".mm:",
	".lua:", ".dart:", ".clj:", ".sh:", ".bash:", ".zig:", ".vue:", ".svelte:",
}

// nodeInlineAnchorMarkers are the pseudo-file anchors node prints for
// inline scripts (`node -e`) and stdin programs; their frames carry
// line:col positions like real source anchors ("at [eval]:1:7").
var nodeInlineAnchorMarkers = []string{"[eval]:", "[stdin]:"}

func DiagnosticAnchor(line string) string {
	lower := strings.ToLower(line)
	for _, ext := range diagnosticAnchorExtensions {
		if idx := strings.Index(lower, ext); idx >= 0 {
			return expandAnchorToken(line, idx, idx+len(ext))
		}
	}
	return nodeInlineAnchor(line, lower)
}

// nodeInlineAnchor recognizes node's bracketed pseudo-file frames when a
// digit follows the marker, so "[eval]:1:7" anchors but prose containing
// "[eval]:" does not.
func nodeInlineAnchor(line string, lower string) string {
	for _, marker := range nodeInlineAnchorMarkers {
		idx := strings.Index(lower, marker)
		if idx < 0 || idx+len(marker) >= len(line) {
			continue
		}
		if c := line[idx+len(marker)]; c < '0' || c > '9' {
			continue
		}
		return expandAnchorToken(line, idx, idx+len(marker))
	}
	return ""
}

func expandAnchorToken(line string, start int, end int) string {
	for start > 0 && !strings.ContainsRune(" \t([{\"'", rune(line[start-1])) {
		start--
	}
	for end < len(line) && !strings.ContainsRune(" \t)]}\"'", rune(line[end])) {
		end++
	}
	return line[start:end]
}

type CompactLineReducer struct {
	scanner      lineScanner
	maxLines     int
	maxBytes     int
	lines        []string
	extraLines   int
	bytesParsed  int
	pendingLine  string
	pendingCount int
}

func NewCompactLineReducer(maxLines, maxBytes int) *CompactLineReducer {
	if maxLines <= 0 {
		maxLines = 12
	}
	return &CompactLineReducer{
		maxLines: maxLines,
		maxBytes: maxBytes,
		lines:    make([]string, 0, maxLines),
	}
}

func (r *CompactLineReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *CompactLineReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *CompactLineReducer) Result() string {
	r.scanner.Finish(r.ingestLine)
	r.flushPending()
	out := append([]string{}, r.lines...)
	if r.extraLines > 0 {
		out = append(out, fmt.Sprintf("... +%d more lines", r.extraLines))
	}
	return strings.Join(out, "\n")
}

func (r *CompactLineReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *CompactLineReducer) FallbackUsed() bool {
	return false
}

func (r *CompactLineReducer) Preview() string {
	out := append([]string{}, r.lines...)
	if r.pendingLine != "" && len(out) < r.maxLines {
		line := r.pendingLine
		if r.pendingCount > 1 {
			line = fmt.Sprintf("%s (x%d)", line, r.pendingCount)
		}
		out = append(out, line)
	}
	if r.extraLines > 0 {
		out = append(out, "... +more lines")
	}
	return strings.Join(out, "\n")
}

func (r *CompactLineReducer) RecoveryInfo() (string, string, bool) {
	if r.extraLines <= 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", r.extraLines))
}

func (r *CompactLineReducer) consume(chunk []byte) {
	r.bytesParsed += len(chunk)
	r.scanner.Consume(chunk, r.ingestLine)
}

func (r *CompactLineReducer) ingestLine(line string) {
	if line == r.pendingLine {
		r.pendingCount++
		return
	}
	r.flushPending()
	r.pendingLine = line
	r.pendingCount = 1
}

func (r *CompactLineReducer) flushPending() {
	if r.pendingLine == "" {
		return
	}
	line := r.pendingLine
	if r.pendingCount > 1 {
		line = fmt.Sprintf("%s (x%d)", line, r.pendingCount)
	}
	r.recordLine(line)
	r.pendingLine = ""
	r.pendingCount = 0
}

func (r *CompactLineReducer) recordLine(line string) {
	if len(r.lines) >= r.maxLines {
		r.extraLines++
		return
	}
	if r.maxBytes > 0 {
		used := 0
		for _, existing := range r.lines {
			used += len(existing) + 1
		}
		remaining := r.maxBytes - used
		if remaining <= 0 {
			r.extraLines++
			return
		}
		if len(line) > remaining {
			line = clip(line, remaining)
		}
	}
	r.lines = append(r.lines, line)
}
