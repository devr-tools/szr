package javascript_test

import (
	"strings"
	"testing"

	jsfilter "github.com/devr-tools/szr/internal/filters/javascript"
)

func TestSummarizeJSTestTextFallback(t *testing.T) {
	text := strings.Join([]string{
		"FAIL src/foo.test.ts",
		"Error: boom",
		"Tests: 1 failed, 2 passed",
		"Snapshots: 0 total",
	}, "\n")
	summary := jsfilter.SummarizeJSTest(text, 4)
	for _, want := range []string{"FAIL src/foo.test.ts", "Error: boom", "Tests: 1 failed, 2 passed"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
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
	summary := jsfilter.SummarizeJSTest(text, 5)
	for _, want := range []string{"FAIL  src/app.test.ts", "AssertionError: expected true to be false", "Tests: 1 failed, 1 total"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestTextCoverageEdges(t *testing.T) {
	got := jsfilter.SummarizeJSTest(strings.Join([]string{
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

	textFallback := jsfilter.SummarizeJSTest("not json\nstill not json\nFAIL x", 1)
	if !strings.Contains(textFallback, "FAIL x") {
		t.Fatalf("expected plain text fallback, got %q", textFallback)
	}
}

func TestBufferedTextReducer(t *testing.T) {
	reducer := jsfilter.NewBufferedTextReducer(true, true, func(input string) string {
		return jsfilter.SummarizeJSTest(input, 4)
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
