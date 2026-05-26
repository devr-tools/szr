package php_test

import (
	"strings"
	"testing"

	phpfilter "github.com/devr-tools/szr/internal/filters/php"
)

func TestSummarizePHP(t *testing.T) {
	input := strings.Join([]string{
		"Composer could not detect the root package version, defaulting to '1.0.0'.",
		"Problem 1",
		"  - Root composer.json requires php ^8.3 but your php version (8.2.7) does not satisfy that requirement.",
		"There was 1 failure:",
		"1) Tests\\Feature\\ExampleTest::test_example",
		"Failed asserting that false is true.",
		"/app/tests/Feature/ExampleTest.php:14",
	}, "\n")

	got := phpfilter.SummarizePHP(input, 6)
	for _, want := range []string{
		"Failed asserting that false is true.",
		"/app/tests/Feature/ExampleTest.php:14",
		"Problem 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in php summary:\n%s", want, got)
		}
	}
}

func TestSummarizePHPFallback(t *testing.T) {
	got := phpfilter.SummarizePHP("plain\noutput\nwithout\nspecial\nmarkers\n", 2)
	if got != "plain\noutput\n... +3 more lines" {
		t.Fatalf("unexpected php fallback summary: %q", got)
	}
}

func TestPHPRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		"Problem 1",
		"Failed asserting that false is true.",
		"/app/tests/Feature/ExampleTest.php:14",
		"Tests: 1, Assertions: 1, Failures: 1.",
	}, "\n")
	if kind, summary, requireRawCapture := phpfilter.PHPRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected PHP recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
