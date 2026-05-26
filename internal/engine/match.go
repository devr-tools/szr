package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

func (e *Engine) match(inv Invocation) Profile {
	for _, profile := range e.profiles {
		if profile.Match != nil && profile.Match(inv) {
			return profile
		}
	}
	return fallbackProfile()
}

func fallbackProfile() Profile {
	const fallbackMaxLines = 12
	return Profile{
		Name:             "passthrough",
		Description:      "Declarative fallback reducer for unmatched commands, with raw passthrough escape paths.",
		Source:           SourceFallback,
		Confidence:       ConfidenceLow,
		StreamPreference: StreamStdoutFirst,
		Budget:           OutputBudget{MaxLines: fallbackMaxLines, MaxBytes: fallbackMaxLines * 160, MaxTokens: fallbackMaxLines * 32},
		Render: func(_ Invocation, exec Execution) string {
			combined := combineStreams(exec.Stdout, exec.Stderr)
			if combined == "" {
				return ""
			}
			if exec.ExitCode != 0 {
				if rendered := filters.InterestingErrorLines(combined, fallbackMaxLines); strings.TrimSpace(rendered) != "" && rendered != "ok" {
					return rendered
				}
			}
			return filters.CompactLines(combined, fallbackMaxLines)
		},
		ParseBytes: func(exec Execution) int {
			return len(combineStreams(exec.Stdout, exec.Stderr))
		},
		Explain: []string{
			"No specialized profile matched.",
			"Uses declarative fallback reduction for compact previews, while preserving raw passthrough escape paths on failures and tee recovery.",
		},
	}
}
