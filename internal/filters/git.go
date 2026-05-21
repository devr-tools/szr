package filters

import (
	"fmt"
	"strings"
)

func SummarizeGitStatus(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "clean"
	}

	branch := ""
	staged := 0
	unstaged := 0
	untracked := 0
	paths := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			branch = strings.TrimPrefix(line, "## ")
			continue
		}
		if len(line) < 3 {
			continue
		}
		x := line[0]
		y := line[1]
		path := strings.TrimSpace(line[3:])
		if path != "" && len(paths) < 6 {
			paths = append(paths, path)
		}
		switch {
		case x == '?' && y == '?':
			untracked++
		default:
			if x != ' ' {
				staged++
			}
			if y != ' ' {
				unstaged++
			}
		}
	}

	summary := []string{}
	if branch != "" {
		summary = append(summary, branch)
	}
	summary = append(summary, fmt.Sprintf("staged=%d unstaged=%d untracked=%d", staged, unstaged, untracked))
	if len(paths) > 0 {
		summary = append(summary, "files:")
		for _, path := range paths {
			summary = append(summary, "  "+path)
		}
	}
	return strings.Join(summary, "\n")
}

func SummarizeGitLog(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "no commits"
	}
	head := lines
	if len(head) > 10 {
		head = head[:10]
	}
	return fmt.Sprintf("%d commits\n%s", len(lines), strings.Join(head, "\n"))
}

func SummarizeGitDiff(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "no diff"
	}

	fileCount := 0
	additions := 0
	deletions := 0
	summary := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			fileCount++
		}
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			additions++
		} else if strings.HasPrefix(line, "-") {
			deletions++
		}
		if strings.Contains(line, "|") || strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
			summary = append(summary, line)
		}
	}
	if len(summary) > 8 {
		summary = summary[:8]
	}

	header := fmt.Sprintf("files=%d +%d -%d", fileCount, additions, deletions)
	if len(summary) == 0 {
		return header + "\n" + CompactLines(input, 12)
	}
	return header + "\n" + strings.Join(summary, "\n")
}
