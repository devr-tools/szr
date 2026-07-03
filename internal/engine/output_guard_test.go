package engine

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestRenderExecutionPrefersRawOverOversizedCanonicalSummary(t *testing.T) {
	t.Parallel()

	// The summary nearly doubles the tiny raw output: canonical markers must
	// not rescue it - an agent is better served by the raw lines.
	raw := "pkg/a.go:12:TODO\npkg/b.go:8:TODO"
	summary := "2 matches across 2 files\npkg/a.go (1 matches)\npkg/b.go (1 matches)"
	rendered := RenderExecution(Profile{
		Name:   "ripgrep",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{Advanced: config.Advanced{CompressionContract: true, CompactArtifactRefs: true}}, Execution{Stdout: raw}, 12, false)
	if rendered.Text != raw {
		t.Fatalf("expected raw output over oversized canonical summary, got %q", rendered.Text)
	}
}

func TestRenderExecutionKeepsCanonicalSummaryWithinSlack(t *testing.T) {
	t.Parallel()

	// A genuinely compact canonical summary (within 1.25x of raw tokens)
	// still survives the small-output guard.
	raw := "pkg/a.go:12:TODO fix widget\npkg/b.go:8:TODO fix gadget\npkg/c.go:4:TODO fix sprocket\npkg/d.go:2:TODO fix flange"
	summary := "4 matches across 4 files\ndirs: pkg/ (4)"
	rendered := RenderExecution(Profile{
		Name:   "ripgrep",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{Advanced: config.Advanced{CompressionContract: true, CompactArtifactRefs: true}}, Execution{Stdout: raw}, 12, false)
	if rendered.Text != summary {
		t.Fatalf("expected compact canonical summary to survive small-output guard, got %q", rendered.Text)
	}
}

func TestRenderExecutionPrefersRawOverOversizedFindSummary(t *testing.T) {
	t.Parallel()

	// A find "summary" carrying more tokens than the two raw paths it
	// describes must yield to the raw listing.
	raw := "src/a.go\nsrc/b.go"
	summary := "2 matches | ext: .go (2)\ndirs: src/ (2)\nexamples: src/a.go, src/b.go"
	rendered := RenderExecution(Profile{
		Name:   "path-find",
		Render: func(Invocation, Execution) string { return summary },
	}, Invocation{Advanced: config.Advanced{CompressionContract: true, CompactArtifactRefs: true}}, Execution{Stdout: raw}, 12, false)
	if rendered.Text != raw {
		t.Fatalf("expected raw paths over oversized find summary, got %q", rendered.Text)
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
