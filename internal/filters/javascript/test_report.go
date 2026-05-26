package javascript

import (
	"encoding/json"
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
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

	if maxLines < 4 {
		maxLines = 4
	}

	details := []string{}
	for _, suite := range report.TestResults {
		if !jsSuiteFailed(suite) {
			continue
		}
		details = append(details, suite.Name)
		for _, line := range jsSuiteDetails(suite) {
			details = append(details, "  "+clip(line, 160))
		}
	}

	if len(details) >= maxLines {
		details = append(details[:maxLines], "... +more failures")
	}
	out = append(out, details...)
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

	if len(lines) == 0 {
		lines = append(lines, failureHighlights(suite.Message)...)
	}

	return uniqueStrings(lines)
}

func failureHighlights(message string) []string {
	roots := []string{}
	frames := []string{}
	for _, line := range nonEmptyLines(message) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "FAIL "),
			strings.Contains(trimmed, "Expected:"),
			strings.Contains(trimmed, "Received:"),
			strings.Contains(trimmed, "AssertionError"),
			strings.Contains(trimmed, "Error:"),
			strings.Contains(trimmed, ".test."),
			strings.Contains(trimmed, ".spec."):
			roots = append(roots, trimmed)
		case strings.HasPrefix(trimmed, "at "):
			frames = append(frames, trimmed)
		}
	}

	if len(roots) == 0 && len(frames) == 0 {
		interesting := []string{}
		for _, line := range nonEmptyLines(message) {
			interesting = append(interesting, strings.TrimSpace(line))
			if len(interesting) == 2 {
				break
			}
		}
		return uniqueStrings(shared.FoldConsecutiveLines(interesting))
	}

	return uniqueStrings(append(shared.FoldConsecutiveLines(roots), shared.SelectUniqueAnchoredLines(frames, 2)...))
}
