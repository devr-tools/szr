package test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestSummarizeJSTestReportFallbackDetails(t *testing.T) {
	input := `{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/logic.spec.ts","status":"passed","message":"boom line one\nboom line two","assertionResults":[{"ancestorTitles":["logic"],"fullName":"","title":"handles edge","status":"failed","failureMessages":["plain failure line\nsecond line"]}]}]}`

	got := filters.SummarizeJSTest(input, 4)
	for _, want := range []string{"src/logic.spec.ts", "logic handles edge", "plain failure line"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in fallback detail summary:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "... +more failures") {
		t.Fatalf("expected detail budget truncation marker:\n%s", got)
	}
}

func TestSummarizeJSTestReportSuiteMessage(t *testing.T) {
	input := `{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/message.spec.ts","status":"failed","message":"Error: boom\nExpected: 1\nReceived: 2","assertionResults":[]}]}`

	got := filters.SummarizeJSTest(input, 6)
	for _, want := range []string{"src/message.spec.ts", "Error: boom", "Expected: 1", "Received: 2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in suite message summary:\n%s", want, got)
		}
	}
}

func TestSummarizeJSTestTextNoTruncation(t *testing.T) {
	input := strings.Join([]string{
		"FAIL src/math.test.ts",
		"Expected: 2",
		"Received: 3",
	}, "\n")

	got := filters.SummarizeJSTest(input, 5)
	if got != input {
		t.Fatalf("unexpected non-truncated text summary: %q", got)
	}
}
