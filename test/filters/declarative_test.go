package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/filters/declarative"
)

func TestDeclarativeBuiltinsLoad(t *testing.T) {
	t.Parallel()

	names, err := declarative.BuiltinNames()
	if err != nil {
		t.Fatalf("load builtin reducers: %v", err)
	}
	for _, want := range []string{"compact_lines", "interesting_error_lines", "read_minimal"} {
		if !containsString(names, want) {
			t.Fatalf("expected builtin %q in %#v", want, names)
		}
	}
}

func TestDeclarativeReducerExecution(t *testing.T) {
	t.Parallel()

	interesting, err := declarative.ApplyBuiltin("interesting_error_lines", strings.Join([]string{
		"info: warmup",
		"warning: retrying connection",
		"plain progress line",
		"panic: boom",
	}, "\n"), declarative.Options{LineLimit: 2})
	if err != nil {
		t.Fatalf("run interesting_error_lines: %v", err)
	}
	if interesting.Text != "warning: retrying connection\npanic: boom" {
		t.Fatalf("unexpected interesting lines output: %q", interesting.Text)
	}

	tail, err := declarative.Apply(declarative.Spec{
		Name:      "tail_preview",
		Tail:      2,
		DropEmpty: true,
	}, "one\ntwo\nthree\nfour\n", declarative.Options{})
	if err != nil {
		t.Fatalf("run inline tail reducer: %v", err)
	}
	if tail.Text != "... +2 earlier lines\nthree\nfour" {
		t.Fatalf("unexpected tail output: %q", tail.Text)
	}
	if tail.OmittedBefore != 2 || tail.OmittedAfter != 0 {
		t.Fatalf("unexpected tail omission metadata: %#v", tail)
	}
	if tail.RecoverySummary("lines") != "omitted 2 additional lines" {
		t.Fatalf("unexpected tail recovery summary: %q", tail.RecoverySummary("lines"))
	}
}

func TestDeclarativeValidation(t *testing.T) {
	t.Parallel()

	_, err := declarative.Apply(declarative.Spec{
		Name: "invalid",
		Head: -1,
	}, "line", declarative.Options{})
	if err == nil || !strings.Contains(err.Error(), "head must be >= 0") {
		t.Fatalf("expected head validation error, got %v", err)
	}
}

func TestDeclarativeHeadTailCombined(t *testing.T) {
	t.Parallel()

	result, err := declarative.Apply(declarative.Spec{
		Name: "head_tail",
		Head: 2,
		Tail: 1,
	}, "one\ntwo\nthree\nfour\nfive\nsix\n", declarative.Options{})
	if err != nil {
		t.Fatalf("run head+tail reducer: %v", err)
	}
	if result.Text != "one\ntwo\n... +3 more lines\nsix" {
		t.Fatalf("unexpected head+tail output: %q", result.Text)
	}
	if result.TotalLines != 6 || result.VisibleLines != 3 || result.OmittedBefore != 0 || result.OmittedAfter != 3 {
		t.Fatalf("unexpected head+tail metadata: %#v", result)
	}
	if result.RecoverySummary("lines") != "omitted 3 additional lines" {
		t.Fatalf("unexpected head+tail recovery summary: %q", result.RecoverySummary("lines"))
	}

	short, err := declarative.Apply(declarative.Spec{
		Name: "head_tail_short",
		Head: 2,
		Tail: 2,
	}, "one\ntwo\nthree\n", declarative.Options{})
	if err != nil {
		t.Fatalf("run short head+tail reducer: %v", err)
	}
	if short.Text != "one\ntwo\nthree" || short.OmittedCount() != 0 {
		t.Fatalf("expected short input to pass through, got %#v", short)
	}
}

func TestDeclarativeDedupAndFold(t *testing.T) {
	t.Parallel()

	deduped, err := declarative.Apply(declarative.Spec{
		Name:             "dedup",
		DropEmpty:        true,
		DedupConsecutive: true,
	}, "warn\nwarn\nwarn\nready\n", declarative.Options{})
	if err != nil {
		t.Fatalf("run dedup reducer: %v", err)
	}
	if deduped.Text != "warn (x3)\nready" {
		t.Fatalf("unexpected dedup output: %q", deduped.Text)
	}
	if deduped.TotalLines != 2 || deduped.VisibleLines != 2 || deduped.OmittedCount() != 0 {
		t.Fatalf("unexpected dedup metadata: %#v", deduped)
	}

	folded, err := declarative.Apply(declarative.Spec{
		Name:        "fold",
		DropEmpty:   true,
		FoldSimilar: true,
	}, strings.Join([]string{
		"2026-06-21T08:00:00Z downloading cache",
		"2026-06-21T08:00:01Z downloading cache",
		"2026-06-21T08:00:02Z downloading cache",
		"retry attempt 1",
		"retry attempt 2",
		"error: build failed",
	}, "\n"), declarative.Options{})
	if err != nil {
		t.Fatalf("run fold reducer: %v", err)
	}
	if folded.Text != "2026-06-21T08:00:00Z downloading cache (x3)\nretry attempt 1 (x2)\nerror: build failed" {
		t.Fatalf("unexpected fold output: %q", folded.Text)
	}

	foldedHead, err := declarative.Apply(declarative.Spec{
		Name:             "fold_head",
		DropEmpty:        true,
		DedupConsecutive: true,
		Head:             1,
	}, "same\nsame\nother\nlast\n", declarative.Options{})
	if err != nil {
		t.Fatalf("run fold+head reducer: %v", err)
	}
	if foldedHead.Text != "same (x2)\n... +2 more lines" {
		t.Fatalf("unexpected fold+head output: %q", foldedHead.Text)
	}
	if foldedHead.TotalLines != 3 || foldedHead.VisibleLines != 1 || foldedHead.OmittedAfter != 2 {
		t.Fatalf("unexpected fold+head metadata: %#v", foldedHead)
	}
}

func TestCompactLinesFoldsRepeats(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("connection refused; retrying\n", 40) + "fatal: giving up\n"
	if got := filters.CompactLines(input, 12); got != "connection refused; retrying (x40)\nfatal: giving up" {
		t.Fatalf("unexpected folded compact lines: %q", got)
	}
}

func TestDeclarativeBuiltinBridge(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"line-1",
		"line-2",
		"line-3",
	}, "\n")
	if got := filters.RenderDeclarativeBuiltin("compact_lines", input, 2); got != "line-1\nline-2\n... +1 more lines" {
		t.Fatalf("unexpected declarative builtin render: %q", got)
	}

	if kind, summary, requireRawCapture := filters.DeclarativeBuiltinRecoveryInfo("compact_lines", "lines", input, 2); kind != filters.RecoveryKindFullOutput || summary != "omitted 1 additional line" || !requireRawCapture {
		t.Fatalf("unexpected declarative builtin recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	reducer := filters.NewDeclarativeBuiltinReducer("compact_lines", "lines", 2, true, false)
	reducer.ConsumeStdout([]byte(input))
	if got := reducer.Result(); got != "line-1\nline-2\n... +1 more lines" {
		t.Fatalf("unexpected declarative reducer result: %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 1 additional line" || !requireRawCapture {
		t.Fatalf("unexpected declarative reducer recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestDeclarativeCompactLinesStreamSemantics(t *testing.T) {
	t.Parallel()

	reducer := filters.NewDeclarativeBuiltinReducer("compact_lines", "lines", 3, true, true)
	reducer.ConsumeStdout([]byte("dup\ndup\n"))
	reducer.ConsumeStderr([]byte("err: one\nerr: two\nerr: three\nerr: four\n"))

	if got := reducer.Result(); got != "dup (x2)\nerr: one\nerr: two\n... +2 more lines" {
		t.Fatalf("unexpected compact lines stream output: %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected compact lines stream recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestDeclarativeCompactLinesStreamFoldsAcrossChunks(t *testing.T) {
	t.Parallel()

	reducer := filters.NewDeclarativeBuiltinReducer("compact_lines", "lines", 12, true, false)
	reducer.ConsumeStdout([]byte("2026-06-21T08:00:00Z downloading cache\n2026-06-21T08:0"))
	reducer.ConsumeStdout([]byte("0:01Z downloading cache\nworker ready\nworker ready\ndone\n"))

	if got := reducer.Result(); got != "2026-06-21T08:00:00Z downloading cache (x2)\nworker ready (x2)\ndone" {
		t.Fatalf("unexpected folded stream output: %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("expected no recovery for fully visible fold, got kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	batch := filters.RenderDeclarativeBuiltin("compact_lines", strings.Join([]string{
		"2026-06-21T08:00:00Z downloading cache",
		"2026-06-21T08:00:01Z downloading cache",
		"worker ready",
		"worker ready",
		"done",
	}, "\n"), 12)
	if batch != "2026-06-21T08:00:00Z downloading cache (x2)\nworker ready (x2)\ndone" {
		t.Fatalf("expected batch output to match streamed output, got %q", batch)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
