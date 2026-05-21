package filters

import (
	"fmt"
	"strings"
)

func CompactLines(input string, maxLines int) string {
	reducer := NewCompactLineReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func DedupeLines(input string, maxLines int) string {
	type item struct {
		Text  string
		Count int
	}

	order := []item{}
	index := map[string]int{}
	for _, line := range nonEmptyLines(input) {
		if pos, ok := index[line]; ok {
			order[pos].Count++
			continue
		}
		index[line] = len(order)
		order = append(order, item{Text: line, Count: 1})
	}

	var out []string
	for _, item := range order {
		line := item.Text
		if item.Count > 1 {
			line = fmt.Sprintf("%s (x%d)", line, item.Count)
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	if len(order) > maxLines {
		out = append(out, fmt.Sprintf("... +%d more unique lines", len(order)-maxLines))
	}
	return strings.Join(out, "\n")
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
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	current := strings.TrimSpace(lines[0])
	count := 0
	flush := func() {
		if current == "" || count == 0 {
			return
		}
		if count > 1 {
			out = append(out, fmt.Sprintf("%s (x%d)", current, count))
			return
		}
		out = append(out, current)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == current {
			count++
			continue
		}
		flush()
		current = trimmed
		count = 1
	}
	flush()
	return out
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

func DiagnosticAnchor(line string) string {
	lower := strings.ToLower(line)
	for _, ext := range []string{".go:", ".py:", ".rs:", ".ts:", ".tsx:", ".js:", ".jsx:", ".java:", ".c:", ".cc:", ".cpp:", ".h:", ".hpp:"} {
		idx := strings.Index(lower, ext)
		if idx < 0 {
			continue
		}
		start := idx
		for start > 0 && !strings.ContainsRune(" \t([{\"'", rune(line[start-1])) {
			start--
		}
		end := idx + len(ext)
		for end < len(line) && !strings.ContainsRune(" \t)]}\"'", rune(line[end])) {
			end++
		}
		return line[start:end]
	}
	return ""
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
