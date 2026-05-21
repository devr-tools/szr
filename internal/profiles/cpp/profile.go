package cpp

import (
	"szr/internal/engine"
	shared "szr/internal/filters"
	cppfilter "szr/internal/filters/cpp"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "ctest",
			Description:      "Summarizes CTest failures and preserves failing test names plus output-on-failure diagnostics.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "ctest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command, "--output-on-failure") {
					return inv.Command
				}
				return append(inv.Command, "--output-on-failure")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cppfilter.SummarizeCTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return cppfilter.SummarizeCTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Adds `--output-on-failure` for plain CTest invocations so test failure bodies are available to the reducer.",
				"Preserves failed tests, assertion output, and total test timing without keeping full pass output.",
			},
		},
		{
			Name:             "clang-tooling",
			Description:      "Summarizes clang tooling and compilation-database generation output around actionable file diagnostics.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isClangToolingCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cppfilter.SummarizeClangTooling(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return cppfilter.SummarizeClangTooling(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches clang analysis and formatting tools plus `bear` compilation-database generation.",
				"Pulls file-level warnings and errors ahead of the surrounding tool chatter.",
			},
		},
	}
}

func isClangToolingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "clang-tidy", "clang-format", "bear":
		return true
	default:
		return false
	}
}
