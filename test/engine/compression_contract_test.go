package engine_test

import (
	"context"
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
