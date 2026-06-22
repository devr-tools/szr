package engine

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestRenderExecutionUltraCompactChangesSummaryShape(t *testing.T) {
	t.Parallel()

	raw := "pkg/a.go:12:TODO\npkg/b.go:8:TODO"
	summary := "2 matches across 2 files\npkg/a.go (1 matches)\npkg/b.go (1 matches)\nexamples: pkg/a.go:12, pkg/b.go:8"
	rendered := RenderExecution(Profile{
		Name:   "ripgrep",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{
		UltraCompact: true,
		Advanced:     config.Advanced{CompressionContract: true, CompactArtifactRefs: true},
	}, Execution{Stdout: raw}, 12, false)

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

func TestRenderStreamingOutputUltraCompactPreservesFailureAnchors(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"build failed",
		"noise",
		"error: undefined symbol RenderWidget",
		"src/ui/widget.go:87",
		"rerun with --trace",
	}, "\n")
	profile := Profile{
		Name: "go-build",
		Render: func(Invocation, Execution) string {
			return "build failed\nerror: undefined symbol RenderWidget\nsrc/ui/widget.go:87\nrerun with --trace"
		},
	}
	inv := Invocation{
		UltraCompact: true,
		Advanced:     config.Advanced{CompressionContract: true, CompactArtifactRefs: true},
	}
	exec := Execution{Stdout: "noise", Stderr: raw, ExitCode: 1}

	rendered, _, _ := renderStreamingOutput(profile, inv, exec, nil, ResolveBudget(profile, inv, 12), combineStreams(exec.Stdout, exec.Stderr), false, FastPathDecision{})
	if strings.Count(rendered, "\n") != 1 {
		t.Fatalf("expected two-line ultra-compact failure render, got %q", rendered)
	}
	for _, want := range []string{"build failed", "error:", "src/ui/widget.go:87", "--trace"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ultra-compact failure render, got %q", want, rendered)
		}
	}
}
