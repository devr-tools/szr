package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"szr/internal/engine"
	"szr/internal/filters"
)

func jsProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:        "js-package-test",
			Description: "Detects Jest and Vitest behind package-manager test scripts and forwards structured reporter flags.",
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
				return filters.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			Explain: []string{
				"Inspects the local `package.json` test script to detect `vitest` or `jest` behind `npm`, `pnpm`, and `yarn` wrappers.",
				"Uses package-manager-specific argument forwarding so structured runner output is requested without changing the user-facing command family.",
			},
		},
		{
			Name:        "vitest-json",
			Description: "Requests the Vitest JSON reporter and preserves failing suite details.",
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
				return filters.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			Explain: []string{
				"Prefers Vitest's JSON reporter when the command did not already request another structured mode.",
				"Condenses passing noise and keeps failed suites, test names, assertion lines, and file paths.",
			},
		},
		{
			Name:        "jest-json",
			Description: "Requests Jest JSON output and condenses the report into failing suite signal.",
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
				return filters.SummarizeJSTest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			Explain: []string{
				"Adds Jest's `--json` mode unless the user already asked for JSON or an alternate reporter.",
				"Summarizes failed suites and assertions while collapsing passing chatter.",
			},
		},
	}
}

func isPackageManagerTest(args []string) bool {
	if len(args) < 2 {
		return false
	}

	if args[0] != "npm" && args[0] != "pnpm" && args[0] != "yarn" {
		return false
	}
	if args[1] == "test" {
		return true
	}
	return len(args) >= 3 && args[1] == "run" && args[2] == "test"
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
