package engine_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
)

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
