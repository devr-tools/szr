package git

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
)

func (r *GitSuccessPathReducer) lines() []string {
	lines := make([]string, 0, len(r.stdoutLines)+len(r.stderrLines))
	lines = append(lines, r.stdoutLines...)
	lines = append(lines, r.stderrLines...)
	return lines
}

func (r *GitSuccessPathReducer) input() string {
	return strings.Join(r.lines(), "\n")
}

func summarizeGitSuccess(mode string, inv engine.Invocation, input string) (string, bool, bool) {
	switch mode {
	case "add":
		return summarizeGitAdd(inv, input)
	case "commit":
		return summarizeGitCommit(inv, input)
	case "push":
		return summarizeGitPush(input)
	case "pull":
		return summarizeGitPull(input)
	default:
		return "", false, false
	}
}
