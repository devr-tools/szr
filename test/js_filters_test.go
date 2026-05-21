package test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestSummarizeJSTestJSON(t *testing.T) {
	input := strings.Join([]string{
		"npm notice some banner",
		`{"numPassedTestSuites":2,"numFailedTestSuites":1,"numPassedTests":5,"numFailedTests":2,"numPendingTests":1,"numTodoTests":0,"numTotalTests":8,"success":false,"testResults":[{"name":"src/math.test.ts","status":"failed","message":"","assertionResults":[{"ancestorTitles":["math"],"fullName":"math adds","title":"adds","status":"failed","failureMessages":["AssertionError: expected 2 to be 3\nExpected: 3\nReceived: 2\nat src/math.test.ts:10:4"]},{"ancestorTitles":["math"],"fullName":"math subtracts","title":"subtracts","status":"failed","failureMessages":["Error: mismatch\nExpected: 2\nReceived: 1"]}]},{"name":"src/pass.test.ts","status":"passed","message":"","assertionResults":[{"fullName":"pass works","title":"works","status":"passed","failureMessages":[]}]}]}`,
	}, "\n")

	got := filters.SummarizeJSTest(input, 8)
	for _, want := range []string{
		"suites: pass=2 fail=1",
		"tests: pass=5 fail=2 skip=1 todo=0 total=8",
		"src/math.test.ts",
		"math adds",
		"Expected: 3",
		"Received: 2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in js json summary:\n%s", want, got)
		}
	}
}

func TestSummarizeJSTestTextFallback(t *testing.T) {
	input := strings.Join([]string{
		"PASS src/pass.test.ts",
		"FAIL src/math.test.ts",
		"  ✕ subtracts",
		"Error: expected values to match",
		"Expected: 2",
		"Received: 3",
		"at src/math.test.ts:10:4",
		"Tests: 1 failed, 1 passed",
		"Time: 0.55s",
	}, "\n")

	got := filters.SummarizeJSTest(input, 6)
	for _, want := range []string{"FAIL src/math.test.ts", "Expected: 2", "Received: 3", "Tests: 1 failed, 1 passed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in js text summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "PASS src/pass.test.ts") {
		t.Fatalf("expected pass noise to be removed:\n%s", got)
	}
}

func TestSummarizeJSTestAllPassAndFallback(t *testing.T) {
	allPass := filters.SummarizeJSTest(`{"numPassedTestSuites":1,"numFailedTestSuites":0,"numPassedTests":3,"numFailedTests":0,"numPendingTests":0,"numTodoTests":0,"numTotalTests":3,"success":true,"testResults":[]}`, 4)
	if allPass != "suites: pass=1 fail=0\ntests: pass=3 fail=0 skip=0 todo=0 total=3\nall tests passed" {
		t.Fatalf("unexpected js all-pass summary: %q", allPass)
	}

	fallback := filters.SummarizeJSTest("plain\noutput\nonly", 2)
	if fallback != "plain\noutput\n... +1 more lines" {
		t.Fatalf("unexpected js compact fallback: %q", fallback)
	}
}
