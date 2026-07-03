package engine_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecuteKeepsFinalDisplayWithinCompressionContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, profiles.Builtins(6))

	cases := []struct {
		name    string
		display []string
		output  string
	}{
		{
			name:    "generic-summary",
			display: []string{"summary"},
			output:  benchmarkEngineGenericSummaryLongInput,
		},
		{
			name:    "gh-run-list",
			display: []string{"gh", "run", "list"},
			output:  benchmarkEngineGHRunListLongInput,
		},
		{
			name:    "kubectl-top",
			display: []string{"kubectl", "top", "pods"},
			output:  benchmarkEngineKubectlTopLongInput,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			script := "#!/bin/sh\ncat <<'EOF'\n" + tc.output + "\nEOF\n"
			commandPath := testutil.WriteExecutable(t, root, tc.name+"-emit", script)
			result, err := e.Execute(context.Background(), engine.Invocation{
				Command: []string{commandPath},
				Display: tc.display,
				Cwd:     root,
			}, false)
			if err != nil {
				t.Fatalf("execute %s: %v", tc.name, err)
			}

			rawTokens := history.EstimateTokens(tc.output)
			displayTokens := history.EstimateTokens(result.Display)
			allowedTokens := retainedTokenCap(rawTokens)
			if displayTokens > allowedTokens {
				t.Fatalf("expected final display <= %d tokens, got %d in %q", allowedTokens, displayTokens, result.Display)
			}
			if result.TeePath == "" {
				t.Fatalf("expected tee path for compressed successful output, got %#v", result)
			}
			if !strings.Contains(result.Display, "[recovery: ") && !strings.Contains(result.Display, "[tee: ") && !strings.Contains(result.Display, "[full output saved]") {
				t.Fatalf("expected compact recovery suffix in display, got %q", result.Display)
			}
		})
	}
}

// TestStreamingCompressionContractUsesTrueRawTokensNotPreview reproduces the
// preview-truncation bug: when full capture is off, rawCombined holds only a
// short preview of the raw stream, and the contract used to budget retained
// tokens against that preview (preview/5) instead of the true streamed token
// count (trueRaw/5), crushing perfectly sized reducer summaries.
func TestStreamingCompressionContractUsesTrueRawTokensNotPreview(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	lines := make([]string, 0, 200)
	for i := 1; i <= 200; i++ {
		lines = append(lines, fmt.Sprintf("record-%03d value=%d status=ok", i, i))
	}
	fullOutput := strings.Join(lines, "\n")
	commandPath := testutil.WriteExecutable(t, root, "bigstream-emit", "#!/bin/sh\ncat <<'EOF'\n"+fullOutput+"\nEOF\n")

	summaryLines := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		summaryLines = append(summaryLines, fmt.Sprintf("field-%02d: object keys=%d items=%d", i, i+2, i*3))
	}
	summaryText := strings.Join(summaryLines, "\n")

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{{
		Name:       "structure-preview",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "bigstream"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: summaryText}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"bigstream"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute streaming summary: %v", err)
	}

	// Preconditions that make the regression observable: capture must be
	// preview-truncated and the summary must exceed the old preview-derived
	// retention cap, so the buggy path would demonstrably compress it.
	if len(result.RawCombined) >= len(fullOutput) {
		t.Fatalf("expected preview-limited capture, got %d of %d raw bytes", len(result.RawCombined), len(fullOutput))
	}
	previewCap := retainedTokenCap(history.EstimateTokens(result.RawCombined))
	summaryTokens := history.EstimateTokens(summaryText)
	if summaryTokens <= previewCap {
		t.Fatalf("fixture too small to detect regression: summary=%d tokens, preview cap=%d", summaryTokens, previewCap)
	}

	if got := history.EstimateTokens(result.Display); got <= previewCap {
		t.Fatalf("regression: display crushed to preview-derived cap %d, got %d tokens in %q", previewCap, got, result.Display)
	}
	if result.Display != summaryText {
		t.Fatalf("expected structure summary to survive the compression contract intact, got %q", result.Display)
	}
	if got := history.EstimateTokens(result.Display); got > retainedTokenCap(history.EstimateTokens(fullOutput)) {
		t.Fatalf("expected display within the true-raw retention cap, got %d tokens", got)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 || records[0].RawTokens <= 2*history.EstimateTokens(result.RawCombined) {
		t.Fatalf("expected streamed raw token count to dwarf the capture preview, got %#v", records)
	}
}

// TestExecuteFailingLintDiagnosticsSurviveVerbatim reproduces a benchmark
// loss: a golangci-lint failure with two concrete issues (~61 raw tokens)
// used to render as a bare "..." plus tee pointer because the compression
// contract armed at 40 raw tokens and crushed the failure escape. Small
// failing outputs must reach the caller verbatim.
func TestExecuteFailingLintDiagnosticsSurviveVerbatim(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	issueOne := "calc/calc.go:20:12: Error return value of `fmt.Errorf` is not checked (errcheck)"
	issueTwo := "calc/calc.go:17:6: func unusedHelper is unused (unused)"
	lintOutput := strings.Join([]string{issueOne, issueTwo, "2 issues:", "* errcheck: 1", "* unused: 1"}, "\n")
	commandPath := testutil.WriteExecutable(t, root, "lint-fail", "#!/bin/sh\ncat <<'EOF'\n"+lintOutput+"\nEOF\nexit 1\n")

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, profiles.Builtins(6))

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"golangci-lint", "run", "./calc/"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute failing lint: %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %#v", result)
	}
	for _, want := range []string{issueOne, issueTwo} {
		if !strings.Contains(result.Display, want) {
			t.Fatalf("expected issue line %q verbatim in display, got %q", want, result.Display)
		}
	}
}

// TestExecuteFailureNeverRendersContentFree pins the failure fidelity
// guarantee end to end: even when a profile renders nothing but an ellipsis
// marker for a failing command, the engine must fall back to compact raw
// lines instead of shipping zero signal.
func TestExecuteFailureNeverRendersContentFree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	errorLine := `E0702 memcache.go:265 "Unhandled Error" err="connection refused"`
	spam := make([]string, 0, 6)
	for i := 0; i < 5; i++ {
		spam = append(spam, errorLine)
	}
	spam = append(spam, "The connection to the server localhost:8080 was refused - did you specify the right host or port?")
	commandPath := testutil.WriteExecutable(t, root, "marker-fail", "#!/bin/sh\ncat <<'EOF' >&2\n"+strings.Join(spam, "\n")+"\nEOF\nexit 1\n")

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{{
		Name:       "marker-only",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "marker-fail"
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return "..."
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"marker-fail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute marker-only failure: %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %#v", result)
	}
	if !strings.Contains(result.Display, "connection refused") && !strings.Contains(result.Display, "localhost:8080") {
		t.Fatalf("expected diagnostic content for failing command, got %q", result.Display)
	}
}

// TestExecuteFailureRecoversDiagnosticsFromTeeArtifact reproduces the
// kubectl-nocluster benchmark loss: a stdout-only stream profile never
// buffers stderr, so when the command fails with stderr-only diagnostics the
// in-memory capture is empty and the display collapsed to a bare tee
// pointer. The failure escape must recover content from the tee artifact.
func TestExecuteFailureRecoversDiagnosticsFromTeeArtifact(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	errorLine := `E0702 memcache.go:265 "Unhandled Error" err="connection refused"`
	commandPath := testutil.WriteExecutable(t, root, "stderr-fail", "#!/bin/sh\ncat <<'EOF' >&2\n"+errorLine+"\nEOF\nexit 1\n")

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{{
		Name:             "stdout-only-table",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "stderr-fail"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: ""}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"stderr-fail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute stderr-only failure: %v", err)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %#v", result)
	}
	if !strings.Contains(result.Display, "connection refused") {
		t.Fatalf("expected stderr diagnostics recovered from tee artifact, got %q", result.Display)
	}
}

func TestRenderExecutionUltraCompactCompactsSingleLineAndBlankOutput(t *testing.T) {
	t.Parallel()

	profile := engine.Profile{
		Name: "single-line",
		Render: func(engine.Invocation, engine.Execution) string {
			return "summary line only"
		},
	}

	rendered := engine.RenderExecution(profile, engine.Invocation{UltraCompact: true}, engine.Execution{}, 12, false)
	if rendered.Text != "summary line only" {
		t.Fatalf("expected single-line render to stay compact, got %q", rendered.Text)
	}

	blank := engine.RenderExecution(engine.Profile{
		Name: "blank",
		Render: func(engine.Invocation, engine.Execution) string {
			return " \n\t "
		},
	}, engine.Invocation{UltraCompact: true}, engine.Execution{}, 12, false)
	if blank.Text != "" {
		t.Fatalf("expected blank ultra-compact render to collapse to empty, got %q", blank.Text)
	}
}

func TestRenderExecutionUltraCompactChangesSummaryShape(t *testing.T) {
	t.Parallel()

	raw := "pkg/a.go:12:TODO\npkg/b.go:8:TODO"
	summary := "2 matches across 2 files\npkg/a.go (1 matches)\npkg/b.go (1 matches)\nexamples: pkg/a.go:12, pkg/b.go:8"
	rendered := engine.RenderExecution(engine.Profile{
		Name:   "ripgrep",
		Render: func(engine.Invocation, engine.Execution) string { return summary },
	}, engine.Invocation{
		UltraCompact: true,
		Advanced:     config.Advanced{CompressionContract: true, CompactArtifactRefs: true},
	}, engine.Execution{Stdout: raw}, 12, false)

	if rendered.Text == raw {
		t.Fatalf("expected ultra-compact render to avoid raw fallback, got %q", rendered.Text)
	}
	if rendered.Text == summary {
		t.Fatalf("expected ultra-compact render to change output shape, got %q", rendered.Text)
	}
	if strings.Count(rendered.Text, "\n") != 1 {
		t.Fatalf("expected two-line ultra-compact shape, got %q", rendered.Text)
	}
	for _, want := range []string{"2 matches across 2 files", "pkg/a.go (1 matches)", "... +"} {
		if !strings.Contains(rendered.Text, want) {
			t.Fatalf("expected %q in ultra-compact render, got %q", want, rendered.Text)
		}
	}
}

func TestRenderExecutionUltraCompactPreservesFailureAnchors(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"build failed",
		"noise",
		"error: undefined symbol RenderWidget",
		"src/ui/widget.go:87",
		"panic: stack exploded",
		"rerun with --trace",
	}, "\n")
	profile := engine.Profile{
		Name: "go-build",
		Render: func(engine.Invocation, engine.Execution) string {
			return "build failed\n3 matches across 2 files\nexamples: src/ui/widget.go:87\n[recovery: omitted 3 lines; tee: abc123]"
		},
	}

	rendered := engine.RenderExecution(profile, engine.Invocation{
		UltraCompact: true,
		Advanced:     config.Advanced{CompressionContract: true, CompactArtifactRefs: true},
	}, engine.Execution{Stderr: raw, ExitCode: 1}, 12, false)

	if strings.Count(rendered.Text, "\n") != 1 {
		t.Fatalf("expected two-line ultra-compact failure render, got %q", rendered.Text)
	}
	for _, want := range []string{"build failed", "error:", "src/ui/widget.go:87"} {
		if !strings.Contains(rendered.Text, want) {
			t.Fatalf("expected %q in ultra-compact failure render, got %q", want, rendered.Text)
		}
	}
	if strings.Contains(rendered.Text, "[recovery:") {
		t.Fatalf("expected recovery bookkeeping line to be deprioritized, got %q", rendered.Text)
	}
}
