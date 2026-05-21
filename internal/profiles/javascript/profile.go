package javascript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"szr/internal/engine"
	shared "szr/internal/filters"
	jsfilter "szr/internal/filters/javascript"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "js-package-test",
			Description:      "Detects Jest and Vitest behind package-manager test scripts and forwards structured reporter flags.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isPackageManagerTest(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				runner := detectPackageTestRunner(inv.Cwd)
				if runner == "" || hasStructuredJSArgs(inv.Command, runner) {
					return inv.Command
				}
				return appendPackageManagerArgs(inv.Command, runnerArgs(runner)...)
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
				return inv.Command
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
				return len(inv.Display) > 0 && inv.Display[0] == "vitest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if hasStructuredJSArgs(inv.Command, "vitest") {
					return inv.Command
				}
				return append(inv.Command, runnerArgs("vitest")...)
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
				return len(inv.Display) > 0 && inv.Display[0] == "jest"
			},
			Prepare: func(inv engine.Invocation) []string {
				if hasStructuredJSArgs(inv.Command, "jest") {
					return inv.Command
				}
				return append(inv.Command, runnerArgs("jest")...)
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

func isPackageManagerTest(args []string) bool {
	return len(args) >= 2 &&
		(args[0] == "npm" || args[0] == "pnpm" || args[0] == "yarn") &&
		(args[1] == "test" || len(args) >= 3 && args[1] == "run" && args[2] == "test")
}

func isJSWorkspaceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn":
		return !isPackageManagerTest(args)
	case "turbo", "nx", "vite":
		return true
	default:
		return false
	}
}

func appendPackageManagerArgs(command []string, extra ...string) []string {
	if len(command) == 0 || len(extra) == 0 {
		return command
	}

	if command[0] == "npm" || command[0] == "pnpm" {
		if containsAny(command, "--") {
			return append(command, extra...)
		}
		out := append([]string{}, command...)
		out = append(out, "--")
		return append(out, extra...)
	}

	return append(command, extra...)
}

func runnerArgs(runner string) []string {
	if runner == "jest" {
		return []string{"--json"}
	}
	return []string{"--reporter=json"}
}

func hasStructuredJSArgs(args []string, runner string) bool {
	if runner == "jest" {
		return containsAny(args, "--json") || containsPrefix(args, "--outputFile") || containsPrefix(args, "--reporters")
	}
	return containsAny(args, "--reporter=json", "--reporter", "json") || containsPrefix(args, "--reporter=") || containsPrefix(args, "--outputFile")
}

func detectPackageTestRunner(cwd string) string {
	if cwd == "" {
		return ""
	}

	type packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}

	path := filepath.Join(cwd, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	script := strings.ToLower(strings.TrimSpace(pkg.Scripts["test"]))
	switch {
	case strings.Contains(script, "vitest"):
		return "vitest"
	case strings.Contains(script, "jest"):
		return "jest"
	default:
		return ""
	}
}

func containsAny(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func containsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
