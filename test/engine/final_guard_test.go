package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// Regression: failing builds used to record NEGATIVE token savings. The
// failure escape re-expanded the render toward raw and the tee suffix was
// appended after the small-output guard, so the final display could cost
// more tokens than the raw output it summarized. The final
// never-worse-than-raw guard must cap the finished display (body plus
// suffixes) at raw cost.
func TestExecuteFailingSmallBuildOutputNeverExceedsRawTokens(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	buildFailPath := testutil.WriteExecutable(t, binDir, "buildfail", "#!/bin/sh\n"+
		"echo '# github.com/example/app' >&2\n"+
		"echo './main.go:12:5: undefined: helperFunc' >&2\n"+
		"echo './main.go:44:9: cannot use x (variable of type string) as int value' >&2\n"+
		"exit 1\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	e := engine.New(config.Default(), paths, store, []engine.Profile{{
		Name:       "build-stream",
		Confidence: engine.ConfidenceMedium,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "buildfail"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &fallbackReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{buildFailPath},
		Display: []string{"buildfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute failing build: %v", err)
	}
	if result.ExitCode != 1 || result.TeePath == "" {
		t.Fatalf("expected failing result with persisted artifact, got %#v", result)
	}
	if !strings.Contains(result.Display, "undefined: helperFunc") {
		t.Fatalf("expected diagnostics in display, got %q", result.Display)
	}
	raw := strings.TrimSpace(result.RawCombined)
	if history.EstimateTokens(result.Display) > history.EstimateTokens(raw) {
		t.Fatalf(
			"final display costs more than raw output: %d > %d (%q)",
			history.EstimateTokens(result.Display), history.EstimateTokens(raw), result.Display,
		)
	}

	assertLastRecordSavingsNonNegative(t, store)
}

// A failure render bigger than raw in the unprotected mid-zone (above the
// small-output guard ceiling, below the compression-contract arming
// threshold) must also be capped by the final guard.
func TestExecuteFailingMidSizeRenderNeverExceedsRawTokens(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	// Ten diagnostic lines keep the raw output above the small-output guard
	// ceiling (96 tokens) but below the compression contract's arming
	// threshold (200 raw tokens) — the historically unprotected zone.
	midFailPath := testutil.WriteExecutable(t, binDir, "midfail", "#!/bin/sh\n"+
		"i=1\nwhile [ $i -le 10 ]; do\n"+
		"  echo \"./pkg/handler_$i.go:$i:7: cannot convert value to target type\" >&2\n"+
		"  i=$((i+1))\ndone\nexit 2\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	verbose := strings.Repeat("verbose diagnostic banner with extra framing lines\n", 30)
	e := engine.New(config.Default(), paths, store, []engine.Profile{{
		Name:       "verbose-render",
		Confidence: engine.ConfidenceMedium,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "midfail"
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return verbose
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{midFailPath},
		Display: []string{"midfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute mid-size failure: %v", err)
	}
	raw := strings.TrimSpace(result.RawCombined)
	if result.Display != raw {
		t.Fatalf("expected raw output to replace an over-budget render, got %q", result.Display)
	}

	assertLastRecordSavingsNonNegative(t, store)
}

// Large failing outputs keep the failure-escape expansion: the escape still
// surfaces compacted diagnostics, it just may never grow past raw.
func TestExecuteLargeFailureKeepsEscapeDetailWithinRaw(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	bigFailPath := testutil.WriteExecutable(t, binDir, "bigfail", "#!/bin/sh\n"+
		"i=1\nwhile [ $i -le 80 ]; do\n"+
		"  echo \"module compile step $i produced diagnostics for package number $i\" >&2\n"+
		"  i=$((i+1))\ndone\n"+
		"echo 'error: build constraint violation in final package' >&2\nexit 1\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	e := engine.New(config.Default(), paths, store, []engine.Profile{{
		Name:       "big-build-stream",
		Confidence: engine.ConfidenceLow,
		Budget:     engine.OutputBudget{MaxLines: 4},
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "bigfail"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &fallbackReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{bigFailPath},
		Display: []string{"bigfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute large failure: %v", err)
	}
	if strings.TrimSpace(result.Display) == "" {
		t.Fatalf("expected failure escape to render diagnostics, got %#v", result)
	}
	rawTokens := history.EstimateTokens(strings.TrimSpace(result.RawCombined))
	displayTokens := history.EstimateTokens(result.Display)
	if displayTokens >= rawTokens {
		t.Fatalf("expected compacted failure escape below raw cost, got %d >= %d", displayTokens, rawTokens)
	}

	assertLastRecordSavingsNonNegative(t, store)
}

// The buffered render pipeline (RenderExecution) shares the same final
// invariant: filtered tokens never exceed raw tokens.
func TestRenderExecutionFailureNeverExceedsRawTokens(t *testing.T) {
	t.Parallel()

	raw := ""
	for i := 0; i < 12; i++ {
		raw += "./cmd/tool.go:31:2: declared and not used: leftoverVariableName\n"
	}
	rendered := engine.RenderExecution(engine.Profile{
		Name:       "over-render",
		Confidence: engine.ConfidenceMedium,
		Render: func(engine.Invocation, engine.Execution) string {
			return strings.Repeat("expansive failure narration with framing and hints\n", 40)
		},
	}, engine.Invocation{}, engine.Execution{Stderr: raw, ExitCode: 2}, 12, false)

	if rendered.FilteredTokens > rendered.RawTokens {
		t.Fatalf("expected filtered tokens <= raw tokens, got %d > %d", rendered.FilteredTokens, rendered.RawTokens)
	}
	if rendered.Text != strings.TrimSpace(raw) {
		t.Fatalf("expected raw output to replace an over-budget render, got %q", rendered.Text)
	}
}

func assertLastRecordSavingsNonNegative(t *testing.T, store *history.Store) {
	t.Helper()
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load history: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("expected at least one history record")
	}
	last := records[len(records)-1]
	if last.SavedTokens < 0 || last.FilteredTokens > last.RawTokens {
		t.Fatalf("expected non-negative savings, got %#v", last)
	}
}
