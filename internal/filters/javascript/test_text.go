package javascript

import (
	"fmt"
	"strings"
)

func summarizeJSTestText(input string, maxLines int) string {
	details := []string{}
	summaries := []string{}
	for _, line := range nonEmptyLines(input) {
		trimmed := strings.TrimSpace(line)
		if isInterestingJSTestLine(trimmed) {
			if isJSTestSummaryLine(trimmed) {
				summaries = append(summaries, clip(trimmed, 160))
				continue
			}
			details = append(details, clip(trimmed, 160))
		}
	}

	details = uniqueStrings(details)
	summaries = prioritizeJSTestSummaries(uniqueStrings(summaries))
	lines := append(details, summaries...)
	if len(lines) == 0 {
		return CompactLines(input, maxLines)
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	selected := lines[:maxLines]
	if len(summaries) > 0 && len(details) >= maxLines && maxLines > 1 {
		selected = append(details[:maxLines-1], summaries[0])
	}
	return strings.Join(selected, "\n") + fmt.Sprintf("\n... +%d more lines", len(lines)-len(selected))
}

func isInterestingJSTestLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "PASS "),
		strings.HasPrefix(line, "at "),
		strings.HasPrefix(line, "✓ "),
		strings.HasPrefix(line, "○ "),
		strings.HasPrefix(line, "· "):
		return false
	}

	switch {
	case strings.HasPrefix(line, "FAIL "),
		strings.HasPrefix(line, "Test Suites:"),
		strings.HasPrefix(line, "Tests:"),
		strings.HasPrefix(line, "Snapshots:"),
		strings.HasPrefix(line, "Duration "),
		strings.HasPrefix(line, "Test Files"),
		strings.HasPrefix(line, " FAIL "),
		strings.HasPrefix(line, "✕ "),
		strings.HasPrefix(line, "× "),
		strings.Contains(line, "Expected:"),
		strings.Contains(line, "Received:"),
		strings.Contains(line, "AssertionError"),
		strings.Contains(line, "Error:"),
		strings.Contains(line, ".test."),
		strings.Contains(line, ".spec."):
		return true
	default:
		return false
	}
}

func isJSTestSummaryLine(line string) bool {
	return strings.HasPrefix(line, "Test Suites:") ||
		strings.HasPrefix(line, "Tests:") ||
		strings.HasPrefix(line, "Snapshots:") ||
		strings.HasPrefix(line, "Time:") ||
		strings.HasPrefix(line, "Duration ") ||
		strings.HasPrefix(line, "Test Files")
}

func prioritizeJSTestSummaries(lines []string) []string {
	prioritized := []string{}
	for _, prefix := range []string{"Test Suites:", "Tests:", "Snapshots:", "Test Files", "Duration ", "Time:"} {
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				prioritized = append(prioritized, line)
			}
		}
	}
	if len(prioritized) == 0 {
		return lines
	}
	return uniqueStrings(prioritized)
}
