package javascript

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

// summarizeJSTestText condenses plain-text runner output (vitest/jest text
// reporters, mocha spec reporter, and package-manager wrapped runs) while
// guaranteeing that every failing spec name survives ahead of assertion
// details and summary counters. A positive allowance self-caps the render to
// that token budget (see shared.PredictedTokenAllowance).
func summarizeJSTestText(input string, maxLines, allowance int) string {
	collector := &jsTestTextCollector{}
	for _, line := range nonEmptyLines(input) {
		collector.ingest(strings.TrimSpace(line))
	}
	collector.flushPending(false)

	failNames := uniqueStrings(collector.failNames)
	details := uniqueStrings(collector.details)
	summaries := prioritizeJSTestSummaries(uniqueStrings(collector.summaries))

	if len(failNames)+len(details)+len(summaries) == 0 {
		return CompactLines(input, maxLines)
	}
	return shared.FitPriorityLinesWithMarker(
		jsTestPriorityLines(failNames, details, summaries),
		maxLines,
		allowance,
	)
}

// jsTestTextCollector classifies plain-text runner lines into failing spec
// names, assertion details, and run summaries; mocha's bare suite headers
// are merged with the indented test-name lines that follow them.
type jsTestTextCollector struct {
	failNames     []string
	details       []string
	summaries     []string
	pendingHeader string
	pendingParts  []string
}

func (c *jsTestTextCollector) ingest(trimmed string) {
	classified := isJSFailingSpecLine(trimmed) || isMochaFailureHeader(trimmed) ||
		isJSTestSummaryLine(trimmed) || isInterestingJSTestLine(trimmed)
	if c.pendingHeader != "" && c.mergePending(trimmed, classified) {
		return
	}

	switch {
	case isJSFailingSpecLine(trimmed):
		c.failNames = append(c.failNames, clip(trimmed, 160))
	case isMochaFailureHeader(trimmed):
		if strings.HasSuffix(trimmed, ":") {
			c.failNames = append(c.failNames, clip(strings.TrimSuffix(trimmed, ":"), 160))
			return
		}
		c.pendingHeader = trimmed
	case isJSTestSummaryLine(trimmed):
		c.summaries = append(c.summaries, clip(trimmed, 160))
	case isInterestingJSTestLine(trimmed):
		c.details = append(c.details, clip(trimmed, 160))
	}
}

// mergePending folds a line into the pending mocha failure header and
// reports whether the line was consumed by the merge.
func (c *jsTestTextCollector) mergePending(trimmed string, classified bool) bool {
	if classified {
		c.flushPending(false)
		return false
	}
	switch {
	case isJSTestNoiseLine(trimmed):
		c.flushPending(false)
		return false
	case strings.HasSuffix(trimmed, ":"):
		c.pendingParts = append(c.pendingParts, trimmed)
		c.flushPending(true)
		return true
	case len(c.pendingParts) < 3:
		c.pendingParts = append(c.pendingParts, trimmed)
		return true
	default:
		c.flushPending(false)
		return false
	}
}

func (c *jsTestTextCollector) flushPending(withParts bool) {
	if c.pendingHeader == "" {
		return
	}
	name := c.pendingHeader
	if withParts && len(c.pendingParts) > 0 {
		name = strings.TrimSuffix(c.pendingHeader+" "+strings.Join(c.pendingParts, " "), ":")
	}
	c.failNames = append(c.failNames, clip(strings.TrimSuffix(name, ":"), 160))
	c.pendingHeader = ""
	c.pendingParts = nil
}

// jsTestPriorityLines tiers the render candidates for budget fitting: every
// failing spec name outranks the primary summary counters, which outrank
// every assertion detail line. A dropped detail loses one clue; a dropped
// fail name loses a whole failing test.
func jsTestPriorityLines(failNames, details, summaries []string) []shared.PriorityLine {
	out := make([]shared.PriorityLine, 0, len(failNames)+len(details)+len(summaries))
	for _, line := range failNames {
		out = append(out, shared.PriorityLine{Text: line, Tier: 0})
	}
	for _, line := range details {
		out = append(out, shared.PriorityLine{Text: line, Tier: 2})
	}
	for i, line := range summaries {
		tier := 3
		if i == 0 {
			tier = 1
		}
		out = append(out, shared.PriorityLine{Text: line, Tier: tier})
	}
	return out
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
