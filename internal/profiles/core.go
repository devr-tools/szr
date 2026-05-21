package profiles

import (
	"szr/internal/engine"
	"szr/internal/filters"
	"szr/internal/profilekit"
)

func coreProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "go-test-json",
			Description:      "Forces `go test -json` and reports package-level failures only.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "go", "test")
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command[1:], "-json") {
					return inv.Command
				}
				return append(inv.Command, "-json")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGoTestJSON(exec.Stdout)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, false, func(input string) string {
					return filters.SummarizeGoTestJSON(input)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Upgrades `go test` to NDJSON mode.",
				"Collapses passing noise and keeps failed packages, tests, and panic lines.",
			},
		},
		{
			Name:             "go-build",
			Description:      "Drops download noise and focuses on compiler diagnostics.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStderrFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "go", "build") || profilekit.HasCommand(inv.Display, "go", "vet")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGenericFailure(exec.Stderr+"\n"+exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewGenericFailureReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStderrFirst,
			Explain: []string{
				"Treats stderr as primary signal for compiler and vet output.",
				"Surfaces error-bearing lines first and trims boilerplate.",
			},
		},
		{
			Name:             "generic-test",
			Description:      "Generic failure-focused profile for wrapped test commands.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "test"
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGenericFailure(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewGenericFailureReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Uses a keyword-focused fallback for arbitrary test runners.",
				"Best-effort mode when there is no structured parser for the tool.",
			},
		},
		{
			Name:             "generic-summary",
			Description:      "Keeps the first informative lines from long command output.",
			Confidence:       engine.ConfidenceLow,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "summary"
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.CompactLines(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewCompactLineReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Does not attempt tool-specific parsing.",
				"Useful when the user wants a shallow preview before drilling deeper.",
			},
		},
	}
}
