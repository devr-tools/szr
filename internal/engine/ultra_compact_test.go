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

	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	rendered, _, _ := renderStreamingOutput(profile, inv, exec, nil, ResolveBudget(profile, inv, 12), rawCombined, 0, false, FastPathDecision{}, len(rawCombined), false, false, "")
	if strings.Count(rendered, "\n") != 1 {
		t.Fatalf("expected two-line ultra-compact failure render, got %q", rendered)
	}
	for _, want := range []string{"build failed", "error:", "src/ui/widget.go:87", "--trace"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ultra-compact failure render, got %q", want, rendered)
		}
	}
}

func TestUltraCompactHelpersCoverDirectPaths(t *testing.T) {
	t.Parallel()

	if got := applyUltraCompactRender(Invocation{}, Execution{}, "  summary  ", ""); got != "  summary  " {
		t.Fatalf("expected non-ultra-compact render to pass through, got %q", got)
	}
	if got := applyUltraCompactRender(Invocation{UltraCompact: true}, Execution{}, " \n\t ", ""); got != "" {
		t.Fatalf("expected blank ultra-compact render to trim to empty, got %q", got)
	}

	lines := compactNonEmptyLines(" one \r\n\r\n two \n  \nthree ")
	if want := []string{"one", "two", "three"}; strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected compact lines: %#v", lines)
	}
	normalized, ok := normalizedUltraCompactLines(" summary \n detail ")
	if !ok || len(normalized) != 2 || normalized[0] != "summary" || normalized[1] != "detail" {
		t.Fatalf("unexpected normalized lines: %#v ok=%v", normalized, ok)
	}

	if got := renderUltraCompactLines([]string{"summary line only"}, "", 0); got != "summary line only" {
		t.Fatalf("unexpected single-line ultra compact render: %q", got)
	}

	detail := buildUltraCompactDetail([]string{"first", "second"}, 2)
	for _, want := range []string{"first", "second", "... +2 lines"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("expected %q in detail %q", want, detail)
		}
	}
	if got := buildUltraCompactDetail(nil, 0); got != "" {
		t.Fatalf("expected empty detail, got %q", got)
	}
	if got := itoa(-12); got != "-12" {
		t.Fatalf("unexpected itoa result: %q", got)
	}
}

func TestUltraCompactCandidatesPreferAnchorsAndRawFailureLines(t *testing.T) {
	t.Parallel()

	renderedLines := []string{
		"build failed",
		"3 matches across 2 files",
		"examples: src/ui/widget.go:87",
		"[recovery: omitted 3 lines; tee: abc123]",
	}
	raw := strings.Join([]string{
		"noise",
		"error: undefined symbol RenderWidget",
		"src/ui/widget.go:87",
		"panic: stack exploded",
	}, "\n")

	candidates := collectUltraCompactCandidates(renderedLines, raw, 1)
	if len(candidates) < 3 {
		t.Fatalf("expected candidates from rendered and raw lines, got %#v", candidates)
	}

	selected, keptRendered := selectUltraCompactCandidates(candidates, ultraCompactMaxDetails(1))
	if keptRendered == 0 {
		t.Fatalf("expected at least one rendered detail to survive, got %#v", selected)
	}
	joined := strings.Join(selected, " | ")
	for _, want := range []string{"error: undefined symbol RenderWidget", "src/ui/widget.go:87"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected selected details to retain %q, got %q", want, joined)
		}
	}

	rendered := renderUltraCompactLines(renderedLines, raw, 1)
	for _, want := range []string{"build failed", "error:", "src/ui/widget.go:87"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ultra-compact render, got %q", want, rendered)
		}
	}
}

func TestUltraCompactScoringHelpers(t *testing.T) {
	t.Parallel()

	if got := ultraCompactSummaryPatternScore("[recovery: omitted 2 lines]", "[recovery: omitted 2 lines]"); got >= 0 {
		t.Fatalf("expected recovery line penalty, got %d", got)
	}
	if got := ultraCompactSummaryPatternScore("3 matches across 2 files", "3 matches across 2 files"); got <= 0 {
		t.Fatalf("expected summary match bonus, got %d", got)
	}
	if got := ultraCompactFailurePatternScore("error: boom", 1); got <= 0 {
		t.Fatalf("expected failure pattern bonus, got %d", got)
	}
	if got := ultraCompactLineScore("examples: src/main.go:12", 1, 1); got <= 0 {
		t.Fatalf("expected positive line score, got %d", got)
	}
}
