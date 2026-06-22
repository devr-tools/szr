package git

import (
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
)

func parseGitChangeTotals(line string) (int, int, int, bool) {
	if !strings.Contains(line, "file changed") && !strings.Contains(line, "files changed") {
		return 0, 0, 0, false
	}
	files, additions, deletions := 0, 0, 0
	fields := strings.Fields(strings.ReplaceAll(line, ",", " "))
	for i := 0; i < len(fields)-1; i++ {
		value, err := strconv.Atoi(fields[i])
		if err != nil {
			continue
		}
		switch fields[i+1] {
		case "file", "files":
			files = value
		case "insertion(+)", "insertions(+)":
			additions = value
		case "deletion(-)", "deletions(-)":
			deletions = value
		}
	}
	return files, additions, deletions, files > 0
}

func gitSubcommandArgs(inv engine.Invocation) []string {
	args := engine.CanonicalArgsForClassification(inv.Command)
	if len(args) < 2 || args[0] != "git" {
		args = engine.CanonicalArgsForClassification(inv.Display)
	}
	if len(args) < 2 || args[0] != "git" {
		return nil
	}
	return args[2:]
}

func gitAddOptionConsumesValue(arg string) bool {
	switch arg {
	case "--chmod", "--pathspec-from-file":
		return true
	default:
		return false
	}
}

func isBroadGitPathspec(arg string) bool {
	switch arg {
	case ".", ":/", ":.", "*":
		return true
	default:
		return false
	}
}

func isGitPushNoise(line string) bool {
	return strings.HasPrefix(line, "To ") ||
		strings.HasPrefix(line, "branch '") ||
		strings.HasPrefix(line, "Enumerating objects:") ||
		strings.HasPrefix(line, "Counting objects:") ||
		strings.HasPrefix(line, "Compressing objects:") ||
		strings.HasPrefix(line, "Writing objects:") ||
		strings.HasPrefix(line, "Total ") ||
		strings.HasPrefix(line, "remote:")
}

func isGitPullNoise(line string) bool {
	return strings.HasPrefix(line, "From ") ||
		strings.HasPrefix(line, " * branch") ||
		strings.HasPrefix(line, " = [up to date]") ||
		strings.Contains(line, "-> origin/") ||
		strings.HasPrefix(line, "remote:")
}

func minUsedGitPullLines(rangeText, mode string, files int) int {
	used := 0
	if rangeText != "" {
		used++
	}
	if mode != "" {
		used++
	}
	if files > 0 {
		used++
	}
	if used == 0 {
		return 1
	}
	return used
}

func rightOfArrow(line string) string {
	parts := strings.SplitN(line, "->", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func leftOfArrow(line string) string {
	parts := strings.SplitN(line, "->", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func firstToken(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func firstLine(value string) string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func looksLikeGitHash(value string) bool {
	if len(value) < 7 || len(value) > 40 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func isGitRevisionRange(value string) bool {
	parts := strings.SplitN(value, "..", 2)
	return len(parts) == 2 && looksLikeGitHash(parts[0]) && looksLikeGitHash(parts[1])
}
