package javascript

import (
	"fmt"
	"strings"
)

// summarizeJSTestText condenses plain-text runner output (vitest/jest text
// reporters, mocha spec reporter, and package-manager wrapped runs) while
// guaranteeing that every failing spec name survives ahead of assertion
// details and summary counters.
func summarizeJSTestText(input string, maxLines int) string {
	failNames := []string{}
	details := []string{}
	summaries := []string{}

	pendingHeader := ""
	pendingParts := []string{}
	flushPending := func(withParts bool) {
		if pendingHeader == "" {
			return
		}
		name := pendingHeader
		if withParts && len(pendingParts) > 0 {
			name = strings.TrimSuffix(pendingHeader+" "+strings.Join(pendingParts, " "), ":")
		}
		failNames = append(failNames, clip(strings.TrimSuffix(name, ":"), 160))
		pendingHeader = ""
		pendingParts = nil
	}

	for _, line := range nonEmptyLines(input) {
		trimmed := strings.TrimSpace(line)
		classified := isJSFailingSpecLine(trimmed) || isMochaFailureHeader(trimmed) ||
			isJSTestSummaryLine(trimmed) || isInterestingJSTestLine(trimmed)

		if pendingHeader != "" && !classified {
			switch {
			case isJSTestNoiseLine(trimmed):
				flushPending(false)
			case strings.HasSuffix(trimmed, ":"):
				pendingParts = append(pendingParts, trimmed)
				flushPending(true)
				continue
			case len(pendingParts) < 3:
				pendingParts = append(pendingParts, trimmed)
				continue
			default:
				flushPending(false)
			}
		} else if pendingHeader != "" {
			flushPending(false)
		}

		switch {
		case isJSFailingSpecLine(trimmed):
			failNames = append(failNames, clip(trimmed, 160))
		case isMochaFailureHeader(trimmed):
			if strings.HasSuffix(trimmed, ":") {
				failNames = append(failNames, clip(strings.TrimSuffix(trimmed, ":"), 160))
				continue
			}
			pendingHeader = trimmed
		case isJSTestSummaryLine(trimmed):
			summaries = append(summaries, clip(trimmed, 160))
		case isInterestingJSTestLine(trimmed):
			details = append(details, clip(trimmed, 160))
		}
	}
	flushPending(false)

	failNames = uniqueStrings(failNames)
	details = uniqueStrings(details)
	summaries = prioritizeJSTestSummaries(uniqueStrings(summaries))

	lines := append([]string{}, failNames...)
	lines = append(lines, details...)
	lines = append(lines, summaries...)
	if len(lines) == 0 {
		return CompactLines(input, maxLines)
	}
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}

	budget := maxLines
	reserveSummary := len(summaries) > 0 && maxLines > 1
	if reserveSummary {
		budget--
	}
	head := append([]string{}, failNames...)
	head = append(head, details...)
	if len(head) > budget {
		head = head[:budget]
	}
	selected := head
	if reserveSummary {
		selected = append(selected, summaries[0])
	}
	return strings.Join(selected, "\n") + fmt.Sprintf("\n... +%d more lines", len(lines)-len(selected))
}

// isJSFailingSpecLine recognizes lines that carry a failing spec's name in
// the jest/vitest text reporters.
func isJSFailingSpecLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "FAIL "),
		strings.HasPrefix(line, "✕ "),
		strings.HasPrefix(line, "× "),
		strings.HasPrefix(line, "✗ "):
		return true
	case strings.HasPrefix(line, "● "):
		return line != "● Console" && !strings.HasPrefix(line, "● Console ")
	default:
		return false
	}
}

// isMochaFailureHeader recognizes mocha-style numbered failure entries such
// as "3) Cart applies discount code:"; bare suite headers ("3) Cart") are
// merged with the indented test-name lines that follow them.
func isMochaFailureHeader(line string) bool {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	return digits > 0 && strings.HasPrefix(line[digits:], ") ")
}

// isJSTestNoiseLine matches per-test progress markers that must never be
// merged into a pending mocha failure name.
func isJSTestNoiseLine(line string) bool {
	return strings.HasPrefix(line, "✓ ") ||
		strings.HasPrefix(line, "✔ ") ||
		strings.HasPrefix(line, "at ") ||
		strings.HasPrefix(line, "- ") ||
		strings.HasPrefix(line, "+ ")
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
	case isJSTestSummaryLine(line),
		strings.Contains(line, "Expected:"),
		strings.Contains(line, "Received:"),
		strings.Contains(line, "AssertionError"),
		strings.Contains(line, "Error:"),
		strings.HasPrefix(line, "+ expected"),
		strings.HasPrefix(line, "expected "),
		strings.Contains(line, ".test."),
		strings.Contains(line, ".spec."):
		return true
	default:
		return false
	}
}

func isJSTestSummaryLine(line string) bool {
	if isMochaCountSummary(line) {
		return true
	}
	return strings.HasPrefix(line, "Test Suites:") ||
		strings.HasPrefix(line, "Tests:") ||
		strings.HasPrefix(line, "Tests ") ||
		strings.HasPrefix(line, "Snapshots:") ||
		strings.HasPrefix(line, "Time:") ||
		strings.HasPrefix(line, "Duration ") ||
		strings.HasPrefix(line, "Test Files") ||
		strings.Contains(line, "ELIFECYCLE")
}

// isMochaCountSummary matches mocha's "2 passing (45ms)" / "2 failing" lines.
func isMochaCountSummary(line string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 || len(fields) > 3 {
		return false
	}
	for _, r := range fields[0] {
		if r < '0' || r > '9' {
			return false
		}
	}
	switch fields[1] {
	case "passing", "failing", "pending":
		return true
	default:
		return false
	}
}

func prioritizeJSTestSummaries(lines []string) []string {
	prioritized := []string{}
	for _, prefix := range []string{"Test Suites:", "Tests:", "Tests ", "Snapshots:", "Test Files", "Duration ", "Time:"} {
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				prioritized = append(prioritized, line)
			}
		}
	}
	for _, line := range lines {
		if isMochaCountSummary(line) || strings.Contains(line, "ELIFECYCLE") {
			prioritized = append(prioritized, line)
		}
	}
	if len(prioritized) == 0 {
		return lines
	}
	return uniqueStrings(prioritized)
}
