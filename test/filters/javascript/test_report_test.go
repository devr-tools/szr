package javascript_test

import (
	"strings"
	"testing"

	jsfilter "github.com/devr-tools/szr/internal/filters/javascript"
)

func TestSummarizeJSTestJSON(t *testing.T) {
	report := `{"numPassedTestSuites":1,"numFailedTestSuites":1,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3"]}]}]}`
	summary := jsfilter.SummarizeJSTest(report, 6)
	for _, want := range []string{"suites: pass=1 fail=1", "src/math.test.ts", "math subtracts", "Expected: 2", "Received: 3"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestAllPassAndFallback(t *testing.T) {
	allPass := jsfilter.SummarizeJSTest(`{"numPassedTestSuites":1,"numFailedTestSuites":0,"numPassedTests":3,"numFailedTests":0,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":true,"testResults":[]}`, 4)
	for _, want := range []string{
		"suites: pass=1 fail=0",
		"tests: pass=3 fail=0 skip=0 todo=0 total=3",
		"all tests passed",
	} {
		if !strings.Contains(allPass, want) {
			t.Fatalf("expected %q in all-pass summary: %q", want, allPass)
		}
	}

	fallback := jsfilter.SummarizeJSTest("not-json", 3)
	if fallback != "not-json" {
		t.Fatalf("expected plain fallback, got %q", fallback)
	}
}

func TestSummarizeJSTestSuiteMessageAndNoTruncation(t *testing.T) {
	suiteMessage := jsfilter.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/fail.test.ts","status":"failed","message":"Error: suite failed\nat src/fail.test.ts:4:2","assertionResults":[]}]}`, 4)
	if !strings.Contains(suiteMessage, "Error: suite failed") {
		t.Fatalf("expected suite message, got %q", suiteMessage)
	}

	noTruncation := jsfilter.SummarizeJSTest("FAIL src/x.test.ts\nsingle line", 10)
	if noTruncation != "FAIL src/x.test.ts" {
		t.Fatalf("expected filtered failure text, got %q", noTruncation)
	}
}

func TestSummarizeJSTestReportCoverageEdges(t *testing.T) {
	embedded := jsfilter.SummarizeJSTest("banner\n{\"numPassedTests\":1,\"numFailedTests\":0,\"numTotalTests\":1,\"numPassedTestSuites\":1,\"numFailedTestSuites\":0,\"testResults\":[]}\ntrailer", 3)
	if !strings.Contains(embedded, "all tests passed") {
		t.Fatalf("expected embedded json parse, got %q", embedded)
	}

	empty := jsfilter.SummarizeJSTest("", 2)
	if empty != "" {
		t.Fatalf("expected empty js summary, got %q", empty)
	}

	statusOnly := jsfilter.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/status-only.test.ts","status":"failed","message":"","assertionResults":[]}]}`, 1)
	if !strings.Contains(statusOnly, "src/status-only.test.ts") {
		t.Fatalf("expected status-only suite to survive summary, got %q", statusOnly)
	}

	unnamed := jsfilter.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/unnamed.test.ts","status":"passed","message":"","assertionResults":[{"ancestorTitles":[],"fullName":"","title":"","status":"failed","failureMessages":["Error: unnamed failure"]}]}]}`, 4)
	if !strings.Contains(unnamed, "Error: unnamed failure") {
		t.Fatalf("expected unnamed failure detail to survive summary, got %q", unnamed)
	}

	mixed := jsfilter.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":1,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":2,"success":false,"testResults":[{"name":"src/mixed.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["mixed"],"fullName":"mixed passes","title":"passes","status":"passed","failureMessages":[]},{"ancestorTitles":["mixed"],"fullName":"mixed fails","title":"fails","status":"failed","failureMessages":["Error: mixed failure"]}]}]}`, 5)
	if !strings.Contains(mixed, "mixed fails") || strings.Contains(mixed, "mixed passes") {
		t.Fatalf("expected only failing assertion details in mixed suite summary, got %q", mixed)
	}

	noData := jsfilter.SummarizeJSTest("{}", 2)
	if noData != "{}" {
		t.Fatalf("expected no-data report to fall back to compact output, got %q", noData)
	}
}

func TestSummarizeJSTestFoldsRepeatedFrames(t *testing.T) {
	report := `{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3\nat src/math.test.ts:12:3\nat src/other.test.ts:9:1"]}]}]}`
	got := jsfilter.SummarizeJSTest(report, 8)
	if strings.Count(got, "at src/math.test.ts:12:3") != 1 {
		t.Fatalf("expected folded JS stack anchors, got %q", got)
	}
	if !strings.Contains(got, "at src/other.test.ts:9:1") {
		t.Fatalf("expected secondary JS frame retention, got %q", got)
	}
}
