package search

import (
	"szr/internal/engine"
	"szr/internal/filters"
	"szr/internal/profilekit"
)

func Profiles(maxLines int, maxGroups int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "ripgrep",
			Description:      "Normalizes ripgrep into stable line-oriented output and groups matches by file.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isRipgrepCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrep(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeRipgrep(exec.Stdout+"\n"+exec.Stderr, maxGroups, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, true, func(input string) string {
					return filters.SummarizeRipgrep(input, maxGroups, maxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets plain-text `rg` usage and adds filename plus line-number flags when the user did not already request them.",
				"Groups matches by file and falls back to error-focused output when ripgrep fails instead of returning matches.",
			},
		},
	}
}

func isRipgrepCommand(args []string) bool {
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	if profilekit.ContainsAny(args[1:], "--json", "--files", "--files-with-matches", "-l", "--count", "-c", "--count-matches") {
		return false
	}
	return true
}

func prepareRipgrep(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	if !profilekit.ContainsAny(out[1:], "-n", "--line-number") {
		out = append(out[:1], append([]string{"-n"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "-H", "--with-filename") {
		out = append(out[:1], append([]string{"-H"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "--no-heading", "--heading") {
		out = append(out[:1], append([]string{"--no-heading"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "--color", "never", "always", "ansi", "auto") && !profilekit.ContainsPrefix(out[1:], "--color=") {
		out = append(out[:1], append([]string{"--color=never"}, out[1:]...)...)
	}
	return out
}
