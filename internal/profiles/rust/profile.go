package rust

import (
	"szr/internal/engine"
	shared "szr/internal/filters"
	rustfilter "szr/internal/filters/rust"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "cargo-test",
			Description:      "Condenses Cargo test output into failing test names, panic lines, and compiler diagnostics.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "cargo", "test")
			},
			Prepare: func(inv engine.Invocation) []string {
				return ensureCargoMessageFormat(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return rustfilter.SummarizeCargoTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return rustfilter.SummarizeCargoTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Recognizes `cargo test` and requests Cargo's short diagnostic format when the command did not already choose one.",
				"Keeps failing test ids, panic locations, compiler errors, and the final test result line while collapsing pass chatter.",
			},
		},
		{
			Name:             "cargo-build",
			Description:      "Summarizes Cargo build and clippy output around compiler diagnostics and hints.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStderrFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "cargo", "build") || profilekit.HasCommand(inv.Display, "cargo", "clippy")
			},
			Prepare: func(inv engine.Invocation) []string {
				return ensureCargoMessageFormat(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return rustfilter.SummarizeCargoBuild(exec.Stderr+"\n"+exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return rustfilter.SummarizeCargoBuild(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStderrFirst,
			Explain: []string{
				"Matches `cargo build` and `cargo clippy` and nudges them toward short diagnostics when possible.",
				"Prioritizes error and warning headers, file anchors, and help or note lines over compile progress noise.",
			},
		},
	}
}

func ensureCargoMessageFormat(command []string) []string {
	if len(command) == 0 || profilekit.ContainsPrefix(command[1:], "--message-format") {
		return command
	}
	insertAt := len(command)
	for i, arg := range command {
		if arg == "--" {
			insertAt = i
			break
		}
	}
	out := append([]string{}, command[:insertAt]...)
	out = append(out, "--message-format=short")
	return append(out, command[insertAt:]...)
}
