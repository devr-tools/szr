package declarative

import (
	"fmt"
	"regexp"
	"strings"
)

type compiledSpec struct {
	spec  Spec
	keep  []lineMatcher
	strip []lineMatcher
	head  int
	tail  int
}

// applyLineLimit overrides the compiled head/tail budget with a caller
// supplied total line limit while preserving the spec's truncation shape.
func (c *compiledSpec) applyLineLimit(limit int) {
	if limit <= 0 {
		return
	}
	switch {
	case c.tail > 0 && c.head == 0:
		c.tail = limit
	case c.tail > 0 && c.head > 0:
		if c.tail >= limit {
			c.head = 1
			c.tail = limit - 1
		} else {
			c.head = limit - c.tail
		}
	default:
		c.head = limit
	}
}

type matcherKind uint8

const (
	matcherRegex matcherKind = iota
	matcherTrimmedPrefix
)

type lineMatcher struct {
	kind    matcherKind
	literal string
	regex   *regexp.Regexp
}

func Apply(spec Spec, input string, opts Options) (Result, error) {
	compiled, err := compileSpec(spec, opts)
	if err != nil {
		return Result{}, err
	}
	return applyCompiled(compiled, input), nil
}

func applyCompiled(compiled compiledSpec, input string) Result {
	lines := splitLines(input)
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if compiled.spec.DropEmpty && strings.TrimSpace(line) == "" {
			continue
		}
		if len(compiled.keep) > 0 && !matchesAny(compiled.keep, line) {
			continue
		}
		if matchesAny(compiled.strip, line) {
			continue
		}
		filtered = append(filtered, line)
	}

	switch {
	case compiled.spec.FoldSimilar:
		filtered = FoldConsecutiveSimilar(filtered)
	case compiled.spec.DedupConsecutive:
		filtered = FoldConsecutive(filtered)
	}

	if compiled.spec.MaxLineWidth > 0 {
		for i, line := range filtered {
			filtered[i] = clipRunes(line, compiled.spec.MaxLineWidth)
		}
	}

	// Folded runs count as a single line: TotalLines and the omission
	// counters describe the folded view, so RecoverySummary stays truthful.
	result := Result{TotalLines: len(filtered)}
	if len(filtered) == 0 {
		result.Empty = true
		result.Text = compiled.spec.EmptyMessage
		return result
	}

	head, tail := compiled.head, compiled.tail
	var out []string
	switch {
	case head > 0 && tail > 0 && len(filtered) > head+tail:
		middle := len(filtered) - head - tail
		result.OmittedAfter = middle
		result.VisibleLines = head + tail
		out = make([]string, 0, head+tail+1)
		out = append(out, filtered[:head]...)
		out = append(out, fmt.Sprintf("... +%d more lines", middle))
		out = append(out, filtered[len(filtered)-tail:]...)
	case head == 0 && tail > 0 && len(filtered) > tail:
		result.OmittedBefore = len(filtered) - tail
		result.VisibleLines = tail
		out = make([]string, 0, tail+1)
		out = append(out, fmt.Sprintf("... +%d earlier lines", result.OmittedBefore))
		out = append(out, filtered[len(filtered)-tail:]...)
	case head > 0 && tail == 0 && len(filtered) > head:
		result.OmittedAfter = len(filtered) - head
		result.VisibleLines = head
		out = make([]string, 0, head+1)
		out = append(out, filtered[:head]...)
		out = append(out, fmt.Sprintf("... +%d more lines", result.OmittedAfter))
	default:
		result.VisibleLines = len(filtered)
		out = filtered
	}
	result.Text = strings.Join(out, "\n")
	return result
}

func ApplyBuiltin(name string, input string, opts Options) (Result, error) {
	compiled, err := compiledBuiltin(name)
	if err != nil {
		return Result{}, err
	}
	compiled.applyLineLimit(opts.LineLimit)
	return applyCompiled(compiled, input), nil
}

func compileSpec(spec Spec, opts Options) (compiledSpec, error) {
	if err := Validate(spec); err != nil {
		return compiledSpec{}, err
	}
	return compileValidatedSpec(spec, opts)
}

func compileValidatedSpec(spec Spec, opts Options) (compiledSpec, error) {
	compiled := compiledSpec{
		spec: spec,
		head: spec.Head,
		tail: spec.Tail,
	}
	compiled.applyLineLimit(opts.LineLimit)
	var err error
	if compiled.keep, err = compilePatterns(spec.KeepPatterns); err != nil {
		return compiledSpec{}, err
	}
	if compiled.strip, err = compilePatterns(spec.StripPatterns); err != nil {
		return compiledSpec{}, err
	}
	return compiled, nil
}

func compilePatterns(patterns []string) ([]lineMatcher, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]lineMatcher, 0, len(patterns))
	for _, pattern := range patterns {
		if literal, ok := parseTrimmedPrefixPattern(pattern); ok {
			out = append(out, lineMatcher{kind: matcherTrimmedPrefix, literal: literal})
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, lineMatcher{kind: matcherRegex, regex: re})
	}
	return out, nil
}

func matchesAny(patterns []lineMatcher, line string) bool {
	for _, pattern := range patterns {
		if pattern.match(line) {
			return true
		}
	}
	return false
}

func (m lineMatcher) match(line string) bool {
	switch m.kind {
	case matcherTrimmedPrefix:
		return strings.HasPrefix(strings.TrimSpace(line), m.literal)
	case matcherRegex:
		return m.regex != nil && m.regex.MatchString(line)
	default:
		return false
	}
}

func parseTrimmedPrefixPattern(pattern string) (string, bool) {
	switch pattern {
	case "^\\s*//":
		return "//", true
	case "^\\s*#":
		return "#", true
	default:
		return "", false
	}
}

func splitLines(input string) []string {
	normalized := strings.ReplaceAll(input, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	parts := strings.Split(normalized, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func clipRunes(input string, max int) string {
	if max <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "..."
}
