package search

import (
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int, maxGroups int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "ripgrep-files",
			Description:      "Summarizes `rg --files` output into a bounded path list.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return isRipgrepFilesCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrepFiles(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewFindReducer(budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets `rg --files` path-list output and preserves ripgrep ignore behavior unless the user explicitly overrides it.",
				"Summarizes tracked paths as a bounded, deduplicated match list instead of treating the command as a generic fallback.",
			},
		},
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
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewRipgrepReducer(maxGroups, budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets plain-text `rg` usage and adds filename plus line-number flags when the user did not already request them.",
				"Groups matches by file and falls back to error-focused output when ripgrep fails instead of returning matches.",
			},
		},
		{
			Name:             "path-find",
			Description:      "Summarizes plain `find` output into a bounded match list.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isPlainFindCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareFind(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewFindReducer(budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets plain `find` usage that emits path lists without destructive actions or custom executors.",
				"Caps long file discovery output to a counted preview instead of replaying every path line.",
			},
		},
	}
}
