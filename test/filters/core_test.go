package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestLineHelpers(t *testing.T) {
	t.Parallel()

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

	folded := filters.FoldConsecutiveLines([]string{"warn", "warn", "error", "error", "error", "hint"})
	if got := strings.Join(folded, ","); got != "warn (x2),error (x3),hint" {
		t.Fatalf("unexpected folded lines: %q", got)
	}

	anchors := filters.SelectUniqueAnchoredLines([]string{
		"at src/app.ts:10:2",
		"at src/app.ts:10:2",
		"at src/other.ts:4:1",
	}, 3)
	if got := strings.Join(anchors, ","); got != "at src/app.ts:10:2,at src/other.ts:4:1" {
		t.Fatalf("unexpected anchored lines: %q", got)
	}
}

func TestFailureHelpers(t *testing.T) {
	t.Parallel()

	if got := filters.SummarizeGenericFailure("", 3); got != "ok" {
		t.Fatalf("unexpected empty generic failure: %q", got)
	}
	generic := filters.SummarizeGenericFailure("info\nwarning: x\npanic: y\n", 1)
	if generic != "panic: y" {
		t.Fatalf("unexpected generic failure summary: %q", generic)
	}
	fallback := filters.SummarizeGenericFailure("line1\nline2\nline3\n", 2)
	if fallback != "line1\nline2\n... +1 more lines" {
		t.Fatalf("unexpected generic fallback: %q", fallback)
	}

	reducer := filters.NewGenericFailureReducer(2, 0)
	reducer.ConsumeStderr([]byte("\x1b[31mwarning"))
	reducer.ConsumeStdout([]byte(": x\x1b[0m\npanic: y\n"))
	if got := reducer.Result(); got != "panic: y\nwarning: x" {
		t.Fatalf("unexpected streaming generic failure: %q", got)
	}
	if reducer.FallbackUsed() {
		t.Fatal("did not expect streaming reducer to report fallback")
	}
	if reducer.BytesParsed() != len("\x1b[31mwarning")+len(": x\x1b[0m\npanic: y\n") {
		t.Fatalf("unexpected bytes parsed: %d", reducer.BytesParsed())
	}

	stack := filters.SummarizeGenericFailure(strings.Join([]string{
		"warning: retrying connection",
		"warning: retrying connection",
		"panic: nil pointer dereference",
		"at /tmp/app/main.go:42",
		"at /tmp/app/main.go:42",
		"help: rerun with --verbose",
	}, "\n"), 4)
	for _, want := range []string{"panic: nil pointer dereference", "/tmp/app/main.go:42", "help: rerun with --verbose"} {
		if !strings.Contains(stack, want) {
			t.Fatalf("expected %q in ranked failure output:\n%s", want, stack)
		}
	}
	if !strings.Contains(stack, "warning: retrying connection (x2)") {
		t.Fatalf("expected repeated warning folding, got %q", stack)
	}
}

func TestUtilityHelpers(t *testing.T) {
	t.Parallel()

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

	entropyCompact := filters.NewCompactLineReducer(2, 0)
	entropyCompact.ConsumeStdout([]byte("repeat\nrepeat\nrepeat\nunique\n"))
	if got := entropyCompact.Result(); got != "repeat (x3)\nunique" {
		t.Fatalf("unexpected entropy-aware compact lines: %q", got)
	}

	contracted := filters.NewGenericFailureReducerWithContract(3, 0, 1, 1, 1)
	contracted.ConsumeStdout([]byte("panic: boom\nsrc/app.go:12:3\nhelp: rerun with -v\n"))
	if got := contracted.Result(); !strings.Contains(got, "panic: boom") || !strings.Contains(got, "src/app.go:12:3") || !strings.Contains(got, "help: rerun with -v") {
		t.Fatalf("expected contract-preserved failure parts, got %q", got)
	}
}
