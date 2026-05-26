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
		Head: 1,
		Tail: 1,
	}, "line", declarative.Options{})
	if err == nil || !strings.Contains(err.Error(), "head and tail cannot both be set") {
		t.Fatalf("expected head/tail validation error, got %v", err)
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
	reducer.ConsumeStderr([]byte("err-1\nerr-2\n"))

	if got := reducer.Result(); got != "dup\ndup\nerr-1\n... +1 more lines" {
		t.Fatalf("unexpected compact lines stream output: %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 1 additional line" || !requireRawCapture {
		t.Fatalf("unexpected compact lines stream recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
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
