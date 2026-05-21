package profiles

import (
	"strings"

	"szr/internal/engine"
	"szr/internal/filters"
)

func rustProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "cargo-test",
			Description:      "Condenses Cargo test output into failing test names, panic lines, and compiler diagnostics.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           outputBudget(atLeast(maxLines, 12)),
			LatencyBudget:    latencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "cargo", "test")
			},
			Prepare: func(inv engine.Invocation) []string {
				return ensureCargoMessageFormat(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeCargoTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, true, func(input string) string {
					return filters.SummarizeCargoTest(input, maxLines)
				})
			},
			ParseBytes: parseCombined,
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
			Budget:           outputBudget(atLeast(maxLines, 10)),
			LatencyBudget:    latencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "cargo", "build") || hasCommand(inv.Display, "cargo", "clippy")
			},
			Prepare: func(inv engine.Invocation) []string {
				return ensureCargoMessageFormat(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeCargoBuild(exec.Stderr+"\n"+exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, true, func(input string) string {
					return filters.SummarizeCargoBuild(input, maxLines)
				})
			},
			ParseBytes: parseStderrFirst,
			Explain: []string{
				"Matches `cargo build` and `cargo clippy` and nudges them toward short diagnostics when possible.",
				"Prioritizes error and warning headers, file anchors, and help or note lines over compile progress noise.",
			},
		},
	}
}

func ensureCargoMessageFormat(command []string) []string {
	if len(command) == 0 || containsPrefix(command[1:], "--message-format") {
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

func isDockerComposeCommand(args []string, sub string) bool {
	return len(args) >= 3 && args[0] == "docker" && args[1] == "compose" && args[2] == sub
}

func insertAfterDockerSubcommand(command []string, extra ...string) []string {
	if len(command) == 0 || len(extra) == 0 {
		return command
	}

	insertAt := len(command)
	start := 2
	if isDockerComposeCommand(command, "logs") || isDockerComposeCommand(command, "ps") {
		start = 3
	}
	for i := start; i < len(command); i++ {
		arg := command[i]
		if !strings.HasPrefix(arg, "-") {
			insertAt = i
			break
		}
	}
	out := append([]string{}, command[:insertAt]...)
	out = append(out, extra...)
	return append(out, command[insertAt:]...)
}
