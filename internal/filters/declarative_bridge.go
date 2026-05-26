package filters

import (
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/filters/declarative"
)

func RenderDeclarativeBuiltin(name, input string, maxLines int) string {
	if name == "compact_lines" {
		return renderDeclarativeCompactLines(input, maxLines)
	}
	result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err != nil {
		return ""
	}
	return result.Text
}

func DeclarativeBuiltinRecoveryInfo(name, noun, input string, maxLines int) (string, string, bool) {
	if name == "compact_lines" {
		return declarativeCompactLinesRecoveryInfo(noun, input, maxLines)
	}
	result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
	if err != nil {
		return NoRecovery()
	}
	return DeclarativeFullOutputRecovery(result, noun)
}

func NewDeclarativeBuiltinReducer(
	name string,
	noun string,
	maxLines int,
	stdoutEnabled bool,
	stderrEnabled bool,
) engineCompatibleReducer {
	if name == "compact_lines" {
		return newDeclarativeCompactLinesReducer(noun, maxLines, stdoutEnabled, stderrEnabled)
	}
	renderBuiltin := func(input string) string {
		result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
		if err != nil {
			return ""
		}
		return result.Text
	}
	recoveryBuiltin := func(input string) (string, string, bool) {
		result, err := declarative.ApplyBuiltin(name, StripANSI(input), declarative.Options{LineLimit: maxLines})
		if err != nil {
			return NoRecovery()
		}
		return DeclarativeFullOutputRecovery(result, noun)
	}
	return NewBufferedTextReducerWithRecovery(
		stdoutEnabled,
		stderrEnabled,
		renderBuiltin,
		recoveryBuiltin,
	)
}

type engineCompatibleReducer interface {
	ConsumeStdout([]byte)
	ConsumeStderr([]byte)
	Result() string
	BytesParsed() int
	FallbackUsed() bool
	RecoveryInfo() (string, string, bool)
}

type declarativeCompactLinesReducer struct {
	noun          string
	maxLines      int
	stdoutEnabled bool
	stderrEnabled bool
	bytesParsed   int
	stdout        declarativeCompactLinesState
	stderr        declarativeCompactLinesState
}

type declarativeCompactLinesState struct {
	scanner    lineScanner
	lines      []string
	totalLines int
	closed     bool
}

func newDeclarativeCompactLinesReducer(noun string, maxLines int, stdoutEnabled bool, stderrEnabled bool) *declarativeCompactLinesReducer {
	if maxLines <= 0 {
		maxLines = 12
	}
	return &declarativeCompactLinesReducer{
		noun:          noun,
		maxLines:      maxLines,
		stdoutEnabled: stdoutEnabled,
		stderrEnabled: stderrEnabled,
		stdout: declarativeCompactLinesState{
			lines: make([]string, 0, maxLines),
		},
		stderr: declarativeCompactLinesState{
			lines: make([]string, 0, maxLines),
		},
	}
}

func renderDeclarativeCompactLines(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}
	lines := make([]string, 0, maxLines)
	total := 0
	scanStringLines(StripANSI(input), func(line string) {
		if line == "" {
			return
		}
		total++
		if len(lines) < maxLines {
			lines = append(lines, line)
		}
	})
	return formatCompactLinesResult(lines, total)
}

func declarativeCompactLinesRecoveryInfo(noun string, input string, maxLines int) (string, string, bool) {
	if maxLines <= 0 {
		maxLines = 12
	}
	visible := 0
	total := 0
	scanStringLines(StripANSI(input), func(line string) {
		if line == "" {
			return
		}
		total++
		if visible < maxLines {
			visible++
		}
	})
	omitted := total - visible
	if omitted <= 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(omittedSummary(omitted, noun))
}

func (r *declarativeCompactLinesReducer) ConsumeStdout(chunk []byte) {
	if !r.stdoutEnabled {
		return
	}
	r.bytesParsed += len(chunk)
	r.stdout.consume(chunk, r.maxLines)
}

func (r *declarativeCompactLinesReducer) ConsumeStderr(chunk []byte) {
	if !r.stderrEnabled {
		return
	}
	r.bytesParsed += len(chunk)
	r.stderr.consume(chunk, r.maxLines)
}

func (r *declarativeCompactLinesReducer) Result() string {
	r.stdout.finish(r.maxLines)
	r.stderr.finish(r.maxLines)
	return r.formatResult()
}

func (r *declarativeCompactLinesReducer) BytesParsed() int {
	return r.bytesParsed
}

func (r *declarativeCompactLinesReducer) FallbackUsed() bool {
	return false
}

func (r *declarativeCompactLinesReducer) RecoveryInfo() (string, string, bool) {
	r.stdout.finish(r.maxLines)
	r.stderr.finish(r.maxLines)
	omitted := r.totalLines() - r.visibleCount()
	if omitted <= 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(omittedSummary(omitted, r.noun))
}

func (s *declarativeCompactLinesState) consume(chunk []byte, maxLines int) {
	s.scanner.Consume(chunk, func(line string) {
		s.totalLines++
		if len(s.lines) < maxLines {
			s.lines = append(s.lines, line)
		}
	})
}

func (s *declarativeCompactLinesState) finish(maxLines int) {
	if s.closed {
		return
	}
	s.closed = true
	s.scanner.Finish(func(line string) {
		s.totalLines++
		if len(s.lines) < maxLines {
			s.lines = append(s.lines, line)
		}
	})
}

func (r *declarativeCompactLinesReducer) totalLines() int {
	return r.stdout.totalLines + r.stderr.totalLines
}

func (r *declarativeCompactLinesReducer) visibleCount() int {
	count := len(r.stdout.lines)
	if count >= r.maxLines {
		return r.maxLines
	}
	count += len(r.stderr.lines)
	if count >= r.maxLines {
		return r.maxLines
	}
	return count
}

func formatCompactLinesResult(lines []string, total int) string {
	omitted := total - len(lines)
	if omitted < 0 {
		omitted = 0
	}
	if len(lines) == 0 {
		if omitted > 0 {
			return overflowLine(omitted)
		}
		return ""
	}
	var builder strings.Builder
	for i, line := range lines {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
	}
	if omitted > 0 {
		builder.WriteByte('\n')
		builder.WriteString(overflowLine(omitted))
	}
	return builder.String()
}

func (r *declarativeCompactLinesReducer) formatResult() string {
	visibleCount := r.visibleCount()
	omitted := r.totalLines() - visibleCount
	if omitted < 0 {
		omitted = 0
	}
	if visibleCount == 0 {
		if omitted > 0 {
			return overflowLine(omitted)
		}
		return ""
	}

	var builder strings.Builder
	written := 0
	appendLine := func(line string) bool {
		if written >= visibleCount {
			return false
		}
		if written > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(line)
		written++
		return written < visibleCount
	}

	for _, line := range r.stdout.lines {
		if !appendLine(line) {
			break
		}
	}
	if written < visibleCount {
		for _, line := range r.stderr.lines {
			if !appendLine(line) {
				break
			}
		}
	}
	if omitted > 0 {
		builder.WriteByte('\n')
		builder.WriteString(overflowLine(omitted))
	}
	return builder.String()
}

func overflowLine(omitted int) string {
	return omittedSummaryLine(omitted)
}

func omittedSummary(omitted int, noun string) string {
	if noun == "" {
		noun = "lines"
	}
	if omitted == 1 {
		return "omitted 1 additional " + singularNoun(noun)
	}
	return "omitted " + itoa(omitted) + " additional " + noun
}

func omittedSummaryLine(omitted int) string {
	return "... +" + itoa(omitted) + " more lines"
}

func singularNoun(noun string) string {
	if noun == "lines" {
		return "line"
	}
	return noun
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func scanStringLines(input string, emit func(string)) {
	start := 0
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '\n':
			emit(strings.TrimRight(input[start:i], " \t"))
			start = i + 1
		case '\r':
			emit(strings.TrimRight(input[start:i], " \t"))
			if i+1 < len(input) && input[i+1] == '\n' {
				i++
			}
			start = i + 1
		}
	}
	if start < len(input) {
		emit(strings.TrimRight(input[start:], " \t"))
	}
}
