package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestLineHelpers(t *testing.T) {
	if got := filters.CompactLines("a\nb\nc", 2); got != "a\nb\n... +1 more lines" {
		t.Fatalf("unexpected compact lines: %q", got)
	}
	if got := filters.CompactLines("a\nb", 5); got != "a\nb" {
		t.Fatalf("unexpected compact passthrough: %q", got)
	}

	deduped := filters.DedupeLines("a\na\nb\nc\nc\n", 2)
	if deduped != "a (x2)\nb\n... +1 more unique lines" {
		t.Fatalf("unexpected dedupe: %q", deduped)
	}
}

func TestFailureHelpers(t *testing.T) {
	if got := filters.SummarizeGenericFailure("", 3); got != "ok" {
		t.Fatalf("unexpected empty generic failure: %q", got)
	}
	generic := filters.SummarizeGenericFailure("info\nwarning: x\npanic: y\n", 1)
	if generic != "warning: x" {
		t.Fatalf("unexpected generic failure summary: %q", generic)
	}
	fallback := filters.SummarizeGenericFailure("line1\nline2\nline3\n", 2)
	if fallback != "line1\nline2\n... +1 more lines" {
		t.Fatalf("unexpected generic fallback: %q", fallback)
	}

	reducer := filters.NewGenericFailureReducer(2, 0)
	reducer.ConsumeStderr([]byte("\x1b[31mwarning"))
	reducer.ConsumeStdout([]byte(": x\x1b[0m\npanic: y\n"))
	if got := reducer.Result(); got != "warning: x\npanic: y" {
		t.Fatalf("unexpected streaming generic failure: %q", got)
	}
	if reducer.FallbackUsed() {
		t.Fatal("did not expect streaming reducer to report fallback")
	}
	if reducer.BytesParsed() != len("\x1b[31mwarning")+len(": x\x1b[0m\npanic: y\n") {
		t.Fatalf("unexpected bytes parsed: %d", reducer.BytesParsed())
	}
}

func TestUtilityHelpers(t *testing.T) {
	if got := filters.CollapseBlock("plain text"); got != "plain text" {
		t.Fatalf("unexpected collapse fallback: %q", got)
	}
	if got := filters.Clip("abcdef", 3); got != "abc..." {
		t.Fatalf("unexpected clip long: %q", got)
	}
	if got := filters.Clip("abc", 5); got != "abc" {
		t.Fatalf("unexpected clip short: %q", got)
	}
	unique := filters.UniqueStrings([]string{" one ", "", "one", "two"})
	if strings.Join(unique, ",") != "one,two" {
		t.Fatalf("unexpected unique strings: %#v", unique)
	}
	if got := filters.ScannerDedupe([]byte("x\nx\n")); got != "x (x2)" {
		t.Fatalf("unexpected scanner dedupe: %q", got)
	}
	if got := filters.StripANSI("\x1b[31mred\x1b[0m plain"); got != "red plain" {
		t.Fatalf("unexpected ansi strip: %q", got)
	}

	compact := filters.NewCompactLineReducer(2, 0)
	compact.ConsumeStdout([]byte("a\n\x1b[31mb"))
	compact.ConsumeStderr([]byte("2\x1b[0m\nc\n"))
	if got := compact.Result(); got != "a\nb2\n... +1 more lines" {
		t.Fatalf("unexpected streaming compact lines: %q", got)
	}
}
