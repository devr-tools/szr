package git

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

func summarizeGitAdd(inv engine.Invocation, input string) (string, bool, bool) {
	if strings.TrimSpace(input) != "" {
		return "", false, false
	}
	pathspecs, broad := gitAddPathspecs(inv)
	switch {
	case broad:
		return "staged all changes", true, false
	case len(pathspecs) == 0:
		return "staged changes", true, false
	default:
		return "staged " + strings.Join(summarizePathBuckets(pathspecs), ", "), true, false
	}
}

//nolint:maintidx // Success-path commit parsing is kept linear so git output handling stays easy to audit.
func summarizeGitCommit(inv engine.Invocation, input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}

	lineIndex := -1
	hash := ""
	subject := ""
	for i, line := range lines {
		hash, subject = parseGitCommitHeadline(line)
		if hash != "" || subject != "" {
			lineIndex = i
			break
		}
	}
	if lineIndex == -1 {
		return "", false, false
	}

	files, additions, deletions := 0, 0, 0
	for _, line := range lines[lineIndex+1:] {
		if parsedFiles, parsedAdditions, parsedDeletions, ok := parseGitChangeTotals(line); ok {
			files, additions, deletions = parsedFiles, parsedAdditions, parsedDeletions
			break
		}
	}

	parts := []string{"committed"}
	if hash != "" {
		parts = append(parts, hash)
	}
	if subject != "" {
		parts = append(parts, subject)
	} else if fallbackSubject := parseGitCommitSubjectFromArgs(inv); fallbackSubject != "" {
		parts = append(parts, fallbackSubject)
	}
	if files > 0 {
		parts = append(parts, fmt.Sprintf("files=%d +%d -%d", files, additions, deletions))
	}
	return strings.Join(parts, " "), true, len(lines) > 1
}

//nolint:maintidx // Pathspec extraction needs to stay close to git's option semantics.
func gitAddPathspecs(inv engine.Invocation) ([]string, bool) {
	args := gitSubcommandArgs(inv)
	pathspecs := make([]string, 0, len(args))
	broad := false
	afterSeparator := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if afterSeparator {
			pathspecs = append(pathspecs, arg)
			if isBroadGitPathspec(arg) {
				broad = true
			}
			continue
		}
		switch {
		case arg == "--":
			afterSeparator = true
		case gitAddOptionConsumesValue(arg):
			i++
		case strings.HasPrefix(arg, "-"):
			if arg == "-A" || arg == "--all" || arg == "-u" || arg == "--update" {
				broad = true
			}
		default:
			pathspecs = append(pathspecs, arg)
			if isBroadGitPathspec(arg) {
				broad = true
			}
		}
	}
	return pathspecs, broad
}

func parseGitCommitHeadline(line string) (string, string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return "", ""
	}
	closeIdx := strings.Index(line, "]")
	if closeIdx <= 0 {
		return "", ""
	}
	head := strings.TrimSpace(line[1:closeIdx])
	subject := ""
	if closeIdx+1 < len(line) {
		subject = strings.TrimSpace(line[closeIdx+1:])
	}
	fields := strings.Fields(head)
	for i := len(fields) - 1; i >= 0; i-- {
		if looksLikeGitHash(fields[i]) {
			return fields[i], subject
		}
	}
	return "", subject
}

func parseGitCommitSubjectFromArgs(inv engine.Invocation) string {
	args := gitSubcommandArgs(inv)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-m" || arg == "--message":
			if i+1 < len(args) {
				return firstLine(args[i+1])
			}
		case strings.HasPrefix(arg, "--message="):
			return firstLine(strings.TrimPrefix(arg, "--message="))
		case strings.HasPrefix(arg, "-m") && len(arg) > 2:
			return firstLine(arg[2:])
		}
	}
	return ""
}
