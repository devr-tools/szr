package javascript

import (
	"github.com/devr-tools/szr/internal/engine"
	jsfilter "github.com/devr-tools/szr/internal/filters/javascript"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	testSummary := profilekit.CombinedBufferedContractSummary(maxLines, 12, 35, engine.StreamStdoutFirst, jsfilter.SummarizeJSTestUnderContract)
	toolingSummary := profilekit.CombinedBufferedContractSummary(maxLines, 10, 30, engine.StreamStdoutFirst, jsfilter.SummarizeJSToolingUnderContract)

	return []engine.Profile{
		nodeEvalProfile(maxLines),
		profilekit.WithSummary(engine.Profile{
			Name:        "bun-test",
			Description: "Summarizes `bun test` output around failed suites and assertions.",
			Confidence:  engine.ConfidenceMedium,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
				FastPathBypass:            engine.FastPathBypassSmallOutput,
				AllowFailureEscape:        true,
				RequireFullCapture:        true,
			},
			Match: func(inv engine.Invocation) bool {
				return isBunTestClassification(inv.Classification.Command) || isBunTestClassification(inv.Classification.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareBunTestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
			},
			Explain: []string{
				"Matches direct `bun test` runs and reuses the JS test reducer family.",
				"Keeps failing suites, assertions, and file anchors while collapsing pass noise.",
			},
		}, testSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "js-package-test",
			Description: "Detects Jest and Vitest behind package-manager test scripts and forwards structured reporter flags.",
			Confidence:  engine.ConfidenceMedium,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
				FastPathBypass:            engine.FastPathBypassSmallOutput,
				AllowFailureEscape:        true,
				RequireFullCapture:        true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Command.JavaScript.IsPackageManagerTest || inv.Classification.Display.JavaScript.IsPackageManagerTest
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
			Explain: []string{
				"Inspects the local `package.json` test script to detect `vitest` or `jest` behind `npm`, `pnpm`, and `yarn` wrappers.",
				"Uses package-manager-specific argument forwarding so structured runner output is requested without changing the user-facing command family.",
			},
		}, testSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "js-workspace",
			Description: "Summarizes JavaScript package-manager, workspace, and Vite output around failed tasks and actionable file errors.",
			Confidence:  engine.ConfidenceMedium,
			Capabilities: engine.ProfileCapabilities{
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
				FastPathBypass:            engine.FastPathBypassSmallOutput,
				AllowFailureEscape:        true,
				RequireFullCapture:        true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.JavaScript.IsWorkspaceCommand
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareJSPackageManagerCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
			},
			Explain: []string{
				"Matches general JavaScript package-manager and workspace-tool commands outside the dedicated test-runner profiles.",
				"Surfaces failed tasks, Vite build errors, package-manager failures, and file anchors instead of long install or build chatter.",
			},
		}, toolingSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "vitest-json",
			Description: "Requests the Vitest JSON reporter and preserves failing suite details.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModeStdoutRequired,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
				RequireFullCapture:        true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Command.Head == "vitest" || inv.Classification.Display.Head == "vitest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.JavaScript.Runner == "vitest" && inv.Classification.Command.JavaScript.StructuredMode {
					return prepareVitestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				}
				return prepareVitestCommand(append(inv.Command, runnerArgs("vitest")...), inv.Advanced.AggressivePrepareRewrites)
			},
			Explain: []string{
				"Prefers Vitest's JSON reporter when the command did not already request another structured mode.",
				"Condenses passing noise and keeps failed suites, test names, assertion lines, and file paths.",
			},
		}, testSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "jest-json",
			Description: "Requests Jest JSON output and condenses the report into failing suite signal.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModeStdoutRequired,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
				RequireFullCapture:        true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Command.Head == "jest" || inv.Classification.Display.Head == "jest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.JavaScript.Runner == "jest" && inv.Classification.Command.JavaScript.StructuredMode {
					return prepareJestCommand(inv.Command, inv.Advanced.AggressivePrepareRewrites)
				}
				return prepareJestCommand(append(inv.Command, runnerArgs("jest")...), inv.Advanced.AggressivePrepareRewrites)
			},
			Explain: []string{
				"Adds Jest's `--json` mode unless the user already asked for JSON or an alternate reporter.",
				"Summarizes failed suites and assertions while collapsing passing chatter.",
			},
		}, testSummary),
	}
}

func isBunTestClassification(command engine.ClassifiedCommand) bool {
	return command.Head == "bun" && command.Subcommand == "test"
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
