package filters

import (
	"encoding/json"
	"fmt"
	"strings"
)

type jsTestReport struct {
	NumPassedTestSuites int           `json:"numPassedTestSuites"`
	NumFailedTestSuites int           `json:"numFailedTestSuites"`
	NumPassedTests      int           `json:"numPassedTests"`
	NumFailedTests      int           `json:"numFailedTests"`
	NumPendingTests     int           `json:"numPendingTests"`
	NumTodoTests        int           `json:"numTodoTests"`
	NumTotalTests       int           `json:"numTotalTests"`
	Success             bool          `json:"success"`
	TestResults         []jsTestSuite `json:"testResults"`
}

type jsTestSuite struct {
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	Message          string            `json:"message"`
	AssertionResults []jsAssertionCase `json:"assertionResults"`
}

type jsAssertionCase struct {
	AncestorTitles  []string `json:"ancestorTitles"`
	FullName        string   `json:"fullName"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	FailureMessages []string `json:"failureMessages"`
}

func SummarizeJSTest(input string, maxLines int) string {
	clean := StripANSI(input)
	if report, ok := parseJSTestReport(clean); ok {
		return summarizeJSTestReport(report, maxLines)
	}
	return summarizeJSTestText(clean, maxLines)
}

func parseJSTestReport(input string) (jsTestReport, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return jsTestReport{}, false
	}

	if start := strings.Index(trimmed, "{"); start >= 0 {
		if end := strings.LastIndex(trimmed, "}"); end > start {
			trimmed = trimmed[start : end+1]
		}
	}

	var report jsTestReport
	if err := json.Unmarshal([]byte(trimmed), &report); err != nil {
		return jsTestReport{}, false
	}
	if report.NumTotalTests == 0 && len(report.TestResults) == 0 {
		return jsTestReport{}, false
	}
	return report, true
}

func summarizeJSTestReport(report jsTestReport, maxLines int) string {
	out := []string{
		fmt.Sprintf("suites: pass=%d fail=%d", report.NumPassedTestSuites, report.NumFailedTestSuites),
		fmt.Sprintf("tests: pass=%d fail=%d skip=%d todo=%d total=%d", report.NumPassedTests, report.NumFailedTests, report.NumPendingTests, report.NumTodoTests, report.NumTotalTests),
	}
	if report.Success || report.NumFailedTests == 0 && report.NumFailedTestSuites == 0 {
		out = append(out, "all tests passed")
		return strings.Join(out, "\n")
	}

	detailBudget := maxLines
	if detailBudget < 4 {
		detailBudget = 4
	}

	for _, suite := range report.TestResults {
		if !jsSuiteFailed(suite) {
			continue
		}
		out = append(out, suite.Name)
		detailBudget--
		if detailBudget <= 0 {
			out = append(out, "... +more failures")
			break
		}

		for _, line := range jsSuiteDetails(suite) {
			out = append(out, "  "+clip(line, 160))
			detailBudget--
			if detailBudget <= 0 {
				out = append(out, "... +more failures")
				return strings.Join(out, "\n")
			}
		}
	}

	return strings.Join(out, "\n")
}

func jsSuiteFailed(suite jsTestSuite) bool {
	if strings.EqualFold(suite.Status, "failed") {
		return true
	}
	for _, assertion := range suite.AssertionResults {
		if strings.EqualFold(assertion.Status, "failed") {
			return true
		}
	}
	return false
}

func jsSuiteDetails(suite jsTestSuite) []string {
	lines := []string{}
	for _, assertion := range suite.AssertionResults {
		if !strings.EqualFold(assertion.Status, "failed") {
			continue
		}

		name := strings.TrimSpace(assertion.FullName)
		if name == "" {
			name = strings.TrimSpace(strings.Join(append(assertion.AncestorTitles, assertion.Title), " "))
		}
		if name != "" {
			lines = append(lines, name)
		}

		for _, message := range assertion.FailureMessages {
			lines = append(lines, failureHighlights(message)...)
		}
	}

	if len(lines) == 0 && suite.Message != "" {
		lines = append(lines, failureHighlights(suite.Message)...)
	}

	return uniqueStrings(lines)
}

func failureHighlights(message string) []string {
	interesting := []string{}
	for _, line := range nonEmptyLines(message) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "FAIL "),
			strings.Contains(trimmed, "Expected:"),
			strings.Contains(trimmed, "Received:"),
			strings.Contains(trimmed, "AssertionError"),
			strings.Contains(trimmed, "Error:"),
			strings.Contains(trimmed, ".test."),
			strings.Contains(trimmed, ".spec."),
			strings.HasPrefix(trimmed, "at "):
			interesting = append(interesting, trimmed)
		}
	}

	if len(interesting) == 0 {
		for _, line := range nonEmptyLines(message) {
			interesting = append(interesting, strings.TrimSpace(line))
			if len(interesting) == 2 {
				break
			}
		}
	}

	return uniqueStrings(interesting)
}

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

	selected := make([]string, 0, maxLines)
	remaining := maxLines

	trailerCount := len(summaries)
	if trailerCount > 2 {
		trailerCount = 2
	}
	if len(details) >= remaining-1 {
		trailerCount = 1
	}
	if trailerCount > remaining {
		trailerCount = remaining
	}
	detailBudget := remaining - trailerCount
	if detailBudget > len(details) {
		detailBudget = len(details)
	}
	selected = append(selected, details[:detailBudget]...)
	if trailerCount > 0 {
		selected = append(selected, summaries[:trailerCount]...)
	}
	if len(selected) > maxLines {
		selected = selected[:maxLines]
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
