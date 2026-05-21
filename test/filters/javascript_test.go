package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestSummarizeJSTestJSON(t *testing.T) {
	report := `{"numPassedTestSuites":1,"numFailedTestSuites":1,"numPassedTests":2,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: expect(received).toBe(expected)\nExpected: 2\nReceived: 3\nat src/math.test.ts:12:3"]}]}]}`
	summary := filters.SummarizeJSTest(report, 6)
	for _, want := range []string{"suites: pass=1 fail=1", "src/math.test.ts", "math subtracts", "Expected: 2", "Received: 3"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestTextFallback(t *testing.T) {
	text := strings.Join([]string{
		"FAIL src/foo.test.ts",
		"Error: boom",
		"Tests: 1 failed, 2 passed",
		"Snapshots: 0 total",
	}, "\n")
	summary := filters.SummarizeJSTest(text, 4)
	for _, want := range []string{"FAIL src/foo.test.ts", "Error: boom", "Tests: 1 failed, 2 passed"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestAllPassAndFallback(t *testing.T) {
	allPass := filters.SummarizeJSTest(`{"numPassedTestSuites":1,"numFailedTestSuites":0,"numPassedTests":3,"numFailedTests":0,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":true,"testResults":[]}`, 4)
	for _, want := range []string{
		"suites: pass=1 fail=0",
		"tests: pass=3 fail=0 skip=0 todo=0 total=3",
		"all tests passed",
	} {
		if !strings.Contains(allPass, want) {
			t.Fatalf("expected %q in all-pass summary: %q", want, allPass)
		}
	}

	fallback := filters.SummarizeJSTest("not-json", 3)
	if fallback != "not-json" {
		t.Fatalf("expected plain fallback, got %q", fallback)
	}
}

func TestSummarizeJSTestFallbackDetails(t *testing.T) {
	text := strings.Join([]string{
		" FAIL  src/app.test.ts",
		"AssertionError: expected true to be false",
		"at src/app.test.ts:12:3",
		"Test Suites: 1 failed, 1 total",
		"Tests: 1 failed, 1 total",
	}, "\n")
	summary := filters.SummarizeJSTest(text, 5)
	for _, want := range []string{"FAIL  src/app.test.ts", "AssertionError: expected true to be false", "Tests: 1 failed, 1 total"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestSuiteMessageAndNoTruncation(t *testing.T) {
	suiteMessage := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/fail.test.ts","status":"failed","message":"Error: suite failed\nat src/fail.test.ts:4:2","assertionResults":[]}]}`, 4)
	if !strings.Contains(suiteMessage, "Error: suite failed") {
		t.Fatalf("expected suite message, got %q", suiteMessage)
	}

	noTruncation := filters.SummarizeJSTest("FAIL src/x.test.ts\nsingle line", 10)
	if noTruncation != "FAIL src/x.test.ts" {
		t.Fatalf("expected filtered failure text, got %q", noTruncation)
	}
}

func TestSummarizeJSTestCoverageEdges(t *testing.T) {
	got := filters.SummarizeJSTest(strings.Join([]string{
		"FAIL src/a.test.ts",
		"Expected: 1",
		"Received: 2",
		"Test Suites: 1 failed",
		"Tests: 1 failed",
		"Snapshots: 0 total",
		"Time: 0.01s",
	}, "\n"), 2)
	for _, want := range []string{"FAIL src/a.test.ts", "Test Suites: 1 failed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in tight js summary:\n%s", want, got)
		}
	}

	embedded := filters.SummarizeJSTest("banner\n{\"numPassedTests\":1,\"numFailedTests\":0,\"numTotalTests\":1,\"numPassedTestSuites\":1,\"numFailedTestSuites\":0,\"testResults\":[]}\ntrailer", 3)
	if !strings.Contains(embedded, "all tests passed") {
		t.Fatalf("expected embedded json parse, got %q", embedded)
	}

	textFallback := filters.SummarizeJSTest("not json\nstill not json\nFAIL x", 1)
	if !strings.Contains(textFallback, "FAIL x") {
		t.Fatalf("expected plain text fallback, got %q", textFallback)
	}

	empty := filters.SummarizeJSTest("", 2)
	if empty != "" {
		t.Fatalf("expected empty js summary, got %q", empty)
	}

	statusOnly := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/status-only.test.ts","status":"failed","message":"","assertionResults":[]}]}`, 1)
	if !strings.Contains(statusOnly, "src/status-only.test.ts") {
		t.Fatalf("expected status-only suite to survive summary, got %q", statusOnly)
	}

	unnamed := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":0,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":1,"success":false,"testResults":[{"name":"src/unnamed.test.ts","status":"passed","message":"","assertionResults":[{"ancestorTitles":[],"fullName":"","title":"","status":"failed","failureMessages":["Error: unnamed failure"]}]}]}`, 4)
	if !strings.Contains(unnamed, "Error: unnamed failure") {
		t.Fatalf("expected unnamed failure detail to survive summary, got %q", unnamed)
	}

	mixed := filters.SummarizeJSTest(`{"numPassedTestSuites":0,"numFailedTestSuites":1,"numPassedTests":1,"numFailedTests":1,"numPendingTests":0,"numTodoTests":0,"numTotalTests":2,"success":false,"testResults":[{"name":"src/mixed.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["mixed"],"fullName":"mixed passes","title":"passes","status":"passed","failureMessages":[]},{"ancestorTitles":["mixed"],"fullName":"mixed fails","title":"fails","status":"failed","failureMessages":["Error: mixed failure"]}]}]}`, 5)
	if !strings.Contains(mixed, "mixed fails") || strings.Contains(mixed, "mixed passes") {
		t.Fatalf("expected only failing assertion details in mixed suite summary, got %q", mixed)
	}

	noData := filters.SummarizeJSTest("{}", 2)
	if noData != "{}" {
		t.Fatalf("expected no-data report to fall back to compact output, got %q", noData)
	}
}

func TestBufferedTextReducer(t *testing.T) {
	reducer := filters.NewBufferedTextReducer(true, true, func(input string) string {
		return filters.SummarizeJSTest(input, 4)
	})
	reducer.ConsumeStdout([]byte("\x1b[31mFAIL src/a.test.ts\x1b[0m\n"))
	reducer.ConsumeStderr([]byte("Error: boom\nTests: 1 failed, 1 total\n"))
	got := reducer.Result()
	for _, want := range []string{"FAIL src/a.test.ts", "Error: boom", "Tests: 1 failed, 1 total"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in buffered reducer output:\n%s", want, got)
		}
	}
	if reducer.BytesParsed() != len("\x1b[31mFAIL src/a.test.ts\x1b[0m\n")+len("Error: boom\nTests: 1 failed, 1 total\n") {
		t.Fatalf("unexpected bytes parsed: %d", reducer.BytesParsed())
	}
}
