package github

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

// ghCommand parses a GitHub CLI or GitLab CLI invocation into its head and
// subcommand. GitLab's `mr` surface mirrors GitHub's `pr` surface, so the
// head is normalized (mr -> pr) and one set of matchers serves both CLIs.
func ghCommand(args []string) (int, int, string, string, bool) {
	if len(args) == 0 || (args[0] != "gh" && args[0] != "glab") {
		return -1, -1, "", "", false
	}
	firstIdx := -1
	for i := 1; i < len(args); {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			firstIdx = i
			break
		}
		i = skipOption(args, i, ghOptionValueFlags)
	}
	if firstIdx == -1 || firstIdx+1 >= len(args) {
		return -1, -1, "", "", false
	}
	head := args[firstIdx]
	if args[0] == "glab" && head == "mr" {
		head = "pr"
	}
	return firstIdx, firstIdx + 1, head, args[firstIdx+1], true
}

func isGitLabCLI(args []string) bool {
	return len(args) > 0 && args[0] == "glab"
}

// prepareGitLabJSONOutput moves a GitLab CLI command into JSON mode unless
// the user already chose an output format.
func prepareGitLabJSONOutput(command []string) []string {
	args := command[1:]
	if containsAny(args, "-F", "--output", "--web") ||
		containsAnyPrefix(args, "--output=", "-F=") {
		return command
	}
	return append(command, "--output", "json")
}

func isGHCommand(args []string, head, sub string) bool {
	_, _, gotHead, gotSub, ok := ghCommand(args)
	return ok && gotHead == head && gotSub == sub
}

func hasGHRunLogFlag(args []string) bool {
	return containsAny(args, "--log", "--log-failed")
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

func newBufferedStdoutReducer(render func(string) string, recovery func(string) (string, string, bool)) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(true, false, render, recovery)
}

func newBufferedCombinedReducer(render func(string) string, recovery func(string) (string, string, bool)) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(true, true, render, recovery)
}

var ghOptionValueFlags = map[string]struct{}{
	"-R":         {},
	"--repo":     {},
	"--hostname": {},
}

func skipOption(args []string, i int, valueFlags map[string]struct{}) int {
	if i >= len(args) {
		return i + 1
	}
	arg := args[i]
	if strings.Contains(arg, "=") {
		return i + 1
	}
	if _, ok := valueFlags[arg]; ok && i+1 < len(args) {
		return i + 2
	}
	return i + 1
}
