package javascript

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	jsfilter "github.com/devr-tools/szr/internal/filters/javascript"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "bun-test",
			Description:      "Summarizes `bun test` output around failed suites and assertions.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Command, "bun", "test") || profilekit.HasCommand(inv.Display, "bun", "test")
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareBunTestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsfilter.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return jsfilter.SummarizeJSTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches direct `bun test` runs and reuses the JS test reducer family.",
				"Keeps failing suites, assertions, and file anchors while collapsing pass noise.",
			},
		},
		{
			Name:             "js-package-test",
			Description:      "Detects Jest and Vitest behind package-manager test scripts and forwards structured reporter flags.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isPackageManagerTest(inv.Command) || isPackageManagerTest(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				if len(inv.Command) == 0 {
					return inv.Command
				}
				runner := detectPackageTestRunner(inv.Cwd, inv.Command)
				if runner == "" {
					return prepareJSPackageManagerCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				}
				command := prepareJSPackageManagerCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				if !hasStructuredJSArgs(command, runner) {
					command = appendPackageManagerArgs(command, runnerArgs(runner)...)
				}
				return prepareJSRunnerCommand(command, runner, inv.Advanced.AggressivePrepareRewrites)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsfilter.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return jsfilter.SummarizeJSTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Inspects the local `package.json` test script to detect `vitest` or `jest` behind `npm`, `pnpm`, and `yarn` wrappers.",
				"Uses package-manager-specific argument forwarding so structured runner output is requested without changing the user-facing command family.",
			},
		},
		{
			Name:             "js-workspace",
			Description:      "Summarizes JavaScript package-manager, workspace, and Vite output around failed tasks and actionable file errors.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isJSWorkspaceCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareJSPackageManagerCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsfilter.SummarizeJSTooling(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return jsfilter.SummarizeJSTooling(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches general JavaScript package-manager and workspace-tool commands outside the dedicated test-runner profiles.",
				"Surfaces failed tasks, Vite build errors, package-manager failures, and file anchors instead of long install or build chatter.",
			},
		},
		{
			Name:             "vitest-json",
			Description:      "Requests the Vitest JSON reporter and preserves failing suite details.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) > 0 && inv.Command[0] == "vitest" || len(inv.Display) > 0 && inv.Display[0] == "vitest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if hasStructuredJSArgs(inv.Command, "vitest") {
					return prepareVitestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				}
				return prepareVitestCommand(append(inv.Command, runnerArgs("vitest")...), inv.Advanced.AggressivePrepareRewrites)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsfilter.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return jsfilter.SummarizeJSTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Prefers Vitest's JSON reporter when the command did not already request another structured mode.",
				"Condenses passing noise and keeps failed suites, test names, assertion lines, and file paths.",
			},
		},
		{
			Name:             "jest-json",
			Description:      "Requests Jest JSON output and condenses the report into failing suite signal.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) > 0 && inv.Command[0] == "jest" || len(inv.Display) > 0 && inv.Display[0] == "jest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if hasStructuredJSArgs(inv.Command, "jest") {
					return prepareJestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				}
				return prepareJestCommand(append(inv.Command, runnerArgs("jest")...), inv.Advanced.AggressivePrepareRewrites)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsfilter.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return jsfilter.SummarizeJSTest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Adds Jest's `--json` mode unless the user already asked for JSON or an alternate reporter.",
				"Summarizes failed suites and assertions while collapsing passing chatter.",
			},
		},
	}
}

func prepareBunTestCommand(command []string, aggressive bool) []string {
	out := append([]string{}, command...)
	if aggressive && !containsAny(command, "--no-color", "--color=false") && !containsPrefix(command, "--color=") {
		out = append(out, "--no-color")
	}
	return out
}

func prepareVitestCommand(command []string, aggressive bool) []string {
	out := append([]string{}, command...)
	if aggressive && !containsAny(command, "--no-color", "--color=false") && !containsPrefix(command, "--color=") {
		out = append(out, "--color=false")
	}
	return out
}

func prepareJestCommand(command []string, aggressive bool) []string {
	out := append([]string{}, command...)
	if aggressive && !containsAny(command, "--color=false", "--colors=false") && !containsPrefix(command, "--color=") && !containsPrefix(command, "--colors=") {
		out = append(out, "--color=false")
	}
	if aggressive && !containsAny(command, "--silent") {
		out = append(out, "--silent")
	}
	return out
}

func prepareJSRunnerCommand(command []string, runner string, aggressive bool) []string {
	switch runner {
	case "jest":
		return prepareJestCommand(command, aggressive)
	case "vitest":
		return prepareVitestCommand(command, aggressive)
	default:
		return command
	}
}

func prepareJSPackageManagerCommand(command []string, aggressive bool) []string {
	if len(command) == 0 {
		return command
	}

	insertAt := jsCommandInsertIndex(command)
	out := append([]string{}, command[:insertAt]...)
	switch command[0] {
	case "npm":
		out = prepareNPMCommand(out, command, aggressive)
	case "pnpm":
		out = preparePNPMCommand(out, command, aggressive)
	case "yarn":
		out = prepareYarnCommand(out, command, aggressive)
	}
	return append(out, command[insertAt:]...)
}

func jsCommandInsertIndex(command []string) int {
	for i, arg := range command {
		if arg == "--" {
			return i
		}
	}
	return len(command)
}

func prepareNPMCommand(out, command []string, aggressive bool) []string {
	out = appendIfAggressive(out, command, aggressive, "--no-progress")
	out = appendIfAggressive(out, command, aggressive, "--no-fund")
	out = appendIfAggressive(out, command, aggressive, "--no-audit")
	return appendJSColorFlag(out, command, aggressive)
}

func preparePNPMCommand(out, command []string, aggressive bool) []string {
	if aggressive && !containsAny(command, "--reporter=silent", "--reporter=append-only") && !containsPrefix(command, "--reporter=") {
		out = append(out, "--reporter=append-only")
	}
	return appendJSColorFlag(out, command, aggressive)
}

func prepareYarnCommand(out, command []string, aggressive bool) []string {
	out = appendIfAggressive(out, command, aggressive, "--silent")
	return appendJSColorFlag(out, command, aggressive)
}

func appendIfAggressive(out, command []string, aggressive bool, arg string) []string {
	if aggressive && !containsAny(command, arg) {
		out = append(out, arg)
	}
	return out
}

func appendJSColorFlag(out, command []string, aggressive bool) []string {
	if aggressive && !containsAny(command, "--color=false") && !containsPrefix(command, "--color=") {
		out = append(out, "--color=false")
	}
	return out
}
