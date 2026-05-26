package declarative

import (
	"fmt"
	"regexp"
	"strings"
)

type compiledSpec struct {
	spec    Spec
	keep    []*regexp.Regexp
	strip   []*regexp.Regexp
	limit   int
	useTail bool
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
		if compiled.spec.MaxLineWidth > 0 {
			line = clipRunes(line, compiled.spec.MaxLineWidth)
		}
		filtered = append(filtered, line)
	}

	result := Result{TotalLines: len(filtered)}
	if len(filtered) == 0 {
		result.Empty = true
		result.Text = compiled.spec.EmptyMessage
		return result
	}

	selected := filtered
	if compiled.limit > 0 && len(filtered) > compiled.limit {
		if compiled.useTail {
			result.OmittedBefore = len(filtered) - compiled.limit
			selected = filtered[len(filtered)-compiled.limit:]
		} else {
			result.OmittedAfter = len(filtered) - compiled.limit
			selected = filtered[:compiled.limit]
		}
	}

	out := append([]string{}, selected...)
	result.VisibleLines = len(out)
	if result.OmittedBefore > 0 {
		out = append([]string{fmt.Sprintf("... +%d earlier lines", result.OmittedBefore)}, out...)
	}
	if result.OmittedAfter > 0 {
		out = append(out, fmt.Sprintf("... +%d more lines", result.OmittedAfter))
	}
	result.Text = strings.Join(out, "\n")
	return result
}

func ApplyBuiltin(name string, input string, opts Options) (Result, error) {
	compiled, err := compiledBuiltin(name)
	if err != nil {
		return Result{}, err
	}
	if opts.LineLimit > 0 && opts.LineLimit != compiled.limit {
		compiled.limit = opts.LineLimit
	}
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
		spec:    spec,
		limit:   spec.Head,
		useTail: spec.Tail > 0,
	}
	if compiled.useTail {
		compiled.limit = spec.Tail
	}
	if opts.LineLimit > 0 {
		compiled.limit = opts.LineLimit
	}
	var err error
	if compiled.keep, err = compilePatterns(spec.KeepPatterns); err != nil {
		return compiledSpec{}, err
	}
	if compiled.strip, err = compilePatterns(spec.StripPatterns); err != nil {
		return compiledSpec{}, err
	}
	return compiled, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

func matchesAny(patterns []*regexp.Regexp, line string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
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
