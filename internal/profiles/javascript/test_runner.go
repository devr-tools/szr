package javascript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

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

func detectPackageTestRunner(cwd string, command []string) string {
	if runner := detectPackageTestRunnerFromPackageJSON(cwd); runner != "" {
		return runner
	}
	return detectPackageTestRunnerFromArgs(command)
}

func detectPackageTestRunnerFromPackageJSON(cwd string) string {
	if cwd == "" {
		return ""
	}

	type packageJSON struct {
		Scripts map[string]string `json:"scripts"`
	}

	for dir := cwd; dir != "" && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		path := filepath.Join(dir, "package.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if next := filepath.Dir(dir); next == dir {
				break
			}
			continue
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
	return ""
}

func detectPackageTestRunnerFromArgs(command []string) string {
	script := strings.ToLower(strings.Join(command, " "))
	switch {
	case strings.Contains(script, "--runinband"):
		return "jest"
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
