package github

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

func ghCommand(args []string) (int, int, string, string, bool) {
	if len(args) == 0 || args[0] != "gh" {
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
	return firstIdx, firstIdx + 1, args[firstIdx], args[firstIdx+1], true
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
