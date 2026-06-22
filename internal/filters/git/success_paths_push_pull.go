package git

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

//nolint:maintidx // Push output varies by remote and ref type, so the parser is intentionally explicit.
func summarizeGitPush(input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}

	if len(lines) == 1 && lines[0] == "Everything up-to-date" {
		return "push up-to-date", true, false
	}

	refSummaries := make([]string, 0, 2)
	ignored := 0
	for _, line := range lines {
		switch {
		case line == "Everything up-to-date":
			return "push up-to-date", true, len(lines) > 1
		case isGitPushNoise(line):
			ignored++
		default:
			summary, ok := parseGitPushRefSummary(line)
			if !ok {
				return "", false, false
			}
			refSummaries = append(refSummaries, summary)
		}
	}
	if len(refSummaries) == 0 {
		return "", false, false
	}
	if len(refSummaries) == 1 {
		return refSummaries[0], true, ignored > 0
	}
	return fmt.Sprintf("pushed %d refs", len(refSummaries)), true, true
}

func summarizeGitPull(input string) (string, bool, bool) {
	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, false
	}
	if summary, ok, ignored := summarizeGitPullUpToDate(lines); ok {
		return summary, true, ignored
	}
	state, ok := parseGitPullState(lines)
	if !ok || !state.usedSignal {
		return "", false, false
	}
	return buildGitPullSummary(state), true, state.ignored > 0 || len(lines) > minUsedGitPullLines(state.rangeText, state.mode, state.files)
}

type gitPullState struct {
	rangeText  string
	mode       string
	files      int
	additions  int
	deletions  int
	usedSignal bool
	ignored    int
}

func summarizeGitPullUpToDate(lines []string) (string, bool, bool) {
	if len(lines) == 1 && lines[0] == "Already up to date." {
		return "pull up-to-date", true, false
	}
	for _, line := range lines {
		if line == "Already up to date." {
			return "pull up-to-date", true, len(lines) > 1
		}
	}
	return "", false, false
}

func parseGitPullState(lines []string) (gitPullState, bool) {
	state := gitPullState{}
	for _, line := range lines {
		if parseGitPullSignal(line, &state) {
			continue
		}
		if isGitPullNoise(line) {
			state.ignored++
			continue
		}
		if !parseGitPullTotals(line, &state) {
			return gitPullState{}, false
		}
	}
	return state, true
}

func parseGitPullSignal(line string, state *gitPullState) bool {
	switch {
	case strings.HasPrefix(line, "Updating "):
		state.rangeText = strings.TrimSpace(strings.TrimPrefix(line, "Updating "))
	case line == "Fast-forward":
		state.mode = "fast-forward"
	case strings.HasPrefix(line, "Merge made by"):
		state.mode = "merge"
	case strings.HasPrefix(line, "Successfully rebased and updated "):
		state.mode = "rebase"
	default:
		return false
	}
	state.usedSignal = true
	return true
}

func parseGitPullTotals(line string, state *gitPullState) bool {
	parsedFiles, parsedAdditions, parsedDeletions, ok := parseGitChangeTotals(line)
	if !ok {
		return false
	}
	state.files = parsedFiles
	state.additions = parsedAdditions
	state.deletions = parsedDeletions
	state.usedSignal = true
	return true
}

func buildGitPullSummary(state gitPullState) string {
	parts := []string{"pulled"}
	if state.rangeText != "" {
		parts = append(parts, state.rangeText)
	}
	if state.mode != "" {
		parts = append(parts, state.mode)
	}
	if state.files > 0 {
		parts = append(parts, fmt.Sprintf("files=%d +%d -%d", state.files, state.additions, state.deletions))
	}
	return strings.Join(parts, " ")
}

func parseGitPushRefSummary(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.Contains(trimmed, "[new branch]"):
		return parseGitPushCreatedRef(trimmed, "new branch")
	case strings.Contains(trimmed, "[new tag]"):
		return parseGitPushCreatedRef(trimmed, "new tag")
	case strings.Contains(trimmed, "->"):
		return parseGitPushUpdatedRef(trimmed)
	default:
		return "", false
	}
}

func parseGitPushCreatedRef(line string, kind string) (string, bool) {
	ref := gitPushArrowRef(line)
	if ref == "" {
		return "", false
	}
	return "pushed " + kind + " " + ref, true
}

func parseGitPushUpdatedRef(line string) (string, bool) {
	left := leftOfArrow(line)
	right := gitPushTargetRef(line)
	if right == "" {
		return "", false
	}
	rangeText := firstToken(left)
	if strings.Contains(line, "(forced update)") {
		return gitPushForcedSummary(right, rangeText), true
	}
	if isGitRevisionRange(rangeText) {
		return fmt.Sprintf("pushed %s %s", right, rangeText), true
	}
	return "pushed " + right, true
}

func gitPushArrowRef(line string) string {
	ref := gitPushTargetRef(line)
	if ref == "" {
		ref = leftOfArrow(line)
	}
	return ref
}

func gitPushTargetRef(line string) string {
	return firstToken(strings.TrimSpace(strings.TrimSuffix(rightOfArrow(line), "(forced update)")))
}

func gitPushForcedSummary(ref string, rangeText string) string {
	if isGitRevisionRange(rangeText) {
		return fmt.Sprintf("force-pushed %s %s", ref, rangeText)
	}
	return "force-pushed " + ref
}
