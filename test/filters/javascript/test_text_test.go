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

func TestSummarizeJSTestTextKeepsVitestFailingSpecNames(t *testing.T) {
	text := strings.Join([]string{
		"> storefront@2.3.1 test /workspace/storefront",
		"> vitest run",
		" ✓ src/lib/__tests__/currency.test.ts (11 tests) 7ms",
		" ❯ src/cart/__tests__/CartSummary.test.tsx (6 tests | 2 failed) 184ms",
		"   ✓ Cart > lists line items",
		"   × Cart > renders empty cart total",
		"   × Cart > applies discount code",
		" FAIL  src/cart/__tests__/CartSummary.test.tsx > Cart > renders empty cart total",
		"AssertionError: expected '$0.01' to be '$0.00' // Object.is equality",
		"Expected: \"$0.00\"",
		"Received: \"$0.01\"",
		" Test Files  1 failed | 4 passed (5)",
		"      Tests  2 failed | 41 passed (43)",
	}, "\n")

	summary := jsfilter.SummarizeJSTest(text, 10)
	for _, want := range []string{
		"× Cart > renders empty cart total",
		"× Cart > applies discount code",
		"AssertionError: expected '$0.01' to be '$0.00'",
		"Tests  2 failed | 41 passed (43)",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in vitest text summary:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "✓ Cart > lists line items") {
		t.Fatalf("expected passing spec noise to be dropped:\n%s", summary)
	}
}

func TestSummarizeJSTestTextKeepsFailingNamesUnderTightBudget(t *testing.T) {
	text := strings.Join([]string{
		"   × Cart > renders empty cart total",
		"   × Cart > applies discount code",
		"AssertionError: expected '$0.01' to be '$0.00'",
		"Expected: \"$0.00\"",
		"Received: \"$0.01\"",
		"      Tests  2 failed | 41 passed (43)",
	}, "\n")

	summary := jsfilter.SummarizeJSTest(text, 3)
	for _, want := range []string{
		"× Cart > renders empty cart total",
		"× Cart > applies discount code",
		"Tests  2 failed | 41 passed (43)",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in tight vitest summary:\n%s", want, summary)
		}
	}
}

func TestSummarizeJSTestTextMochaFailures(t *testing.T) {
	text := strings.Join([]string{
		"  Cart",
		"    ✓ lists line items",
		"    1) renders empty cart total",
		"    2) applies discount code",
		"  41 passing (312ms)",
		"  2 failing",
		"  1) Cart",
		"       renders empty cart total:",
		"     AssertionError: expected '$0.01' to equal '$0.00'",
		"      at Context.<anonymous> (test/cart.test.js:24:31)",
		"  2) Cart",
		"       applies discount code:",
		"     AssertionError: expected 1240 to equal 1116",
	}, "\n")

	summary := jsfilter.SummarizeJSTest(text, 12)
	for _, want := range []string{
		"1) Cart renders empty cart total",
		"2) Cart applies discount code",
		"AssertionError: expected '$0.01' to equal '$0.00'",
		"AssertionError: expected 1240 to equal 1116",
		"2 failing",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("expected %q in mocha summary:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "✓ lists line items") {
		t.Fatalf("expected mocha pass noise to be dropped:\n%s", summary)
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
