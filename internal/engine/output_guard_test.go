package engine

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestRenderExecutionKeepsCanonicalRipgrepSummaryOnSuccess(t *testing.T) {
	t.Parallel()

	raw := "pkg/a.go:12:TODO\npkg/b.go:8:TODO"
	summary := "2 matches across 2 files\npkg/a.go (1 matches)\npkg/b.go (1 matches)"
	rendered := RenderExecution(Profile{
		Name:   "ripgrep",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{Advanced: config.Advanced{CompressionContract: true, CompactArtifactRefs: true}}, Execution{Stdout: raw}, 12, false)
	if rendered.Text != summary {
		t.Fatalf("expected canonical ripgrep summary to survive small-output guard, got %q", rendered.Text)
	}
}

func TestRenderExecutionKeepsCanonicalFindSummaryOnSuccess(t *testing.T) {
	t.Parallel()

	raw := "src/a.go\nsrc/b.go"
	summary := "2 matches | ext: .go (2)\ndirs: src/ (2)\nexamples: src/a.go, src/b.go"
	rendered := RenderExecution(Profile{
		Name:   "path-find",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{Advanced: config.Advanced{CompressionContract: true, CompactArtifactRefs: true}}, Execution{Stdout: raw}, 12, false)
	if rendered.Text != summary {
		t.Fatalf("expected canonical find summary to survive small-output guard, got %q", rendered.Text)
	}
}

func TestPreferRawSmallOutputForProfileKeepsFailureBias(t *testing.T) {
	t.Parallel()

	profile := Profile{Name: "ripgrep"}
	raw := "pkg/a.go:12:TODO"
	rendered := "1 matches across 1 files\npkg/a.go (1 matches)"
	if got := preferRawSmallOutputForProfile(profile, rendered, raw, 1); got != raw {
		t.Fatalf("expected raw output to remain preferred on nonzero exit, got %q", got)
	}
}
