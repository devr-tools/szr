package engine

import (
	"szr/internal/filters"
	"szr/internal/rules"
)

func renderRule(render rules.Render, exec Execution, defaultMaxLines int) string {
	mode := render.Mode
	if mode == "" {
		mode = "compact"
	}
	maxLines := render.MaxLines
	if maxLines == 0 {
		maxLines = defaultMaxLines
	}

	combined := combineStreams(exec.Stdout, exec.Stderr)
	switch mode {
	case "failure":
		return filters.SummarizeGenericFailure(combined, maxLines)
	case "passthrough":
		return combined
	default:
		return filters.CompactLines(combined, maxLines)
	}
}
