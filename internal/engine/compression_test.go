package engine

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/history"
)

func TestCompressionContractAllowedTokens(t *testing.T) {
	t.Parallel()

	if got := compressionContractAllowedTokens(100, OutputBudget{MaxTokens: 50}); got != 20 {
		t.Fatalf("expected 20 retained tokens, got %d", got)
	}
	if got := compressionContractAllowedTokens(50, OutputBudget{MaxTokens: 6}); got != 6 {
		t.Fatalf("expected budget cap to win, got %d", got)
	}
	if got := compressionContractAllowedTokens(40, OutputBudget{}); got != 8 {
		t.Fatalf("expected minimum token floor, got %d", got)
	}
}

func TestEnforceCompressionContractCompressesLargeOutput(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 80)
	text := strings.Repeat("token ", 40)
	compressed, plan, changed := enforceCompressionContract(text, raw, 0, OutputBudget{MaxTokens: 32}, RecoveryPlan{}, false, true)
	if !changed {
		t.Fatal("expected compression contract to apply")
	}
	allowed := compressionContractAllowedTokens(history.EstimateTokens(raw), OutputBudget{MaxTokens: 32})
	if got := history.EstimateTokens(compressed); got > allowed {
		t.Fatalf("expected compressed output to respect token cap %d, got %d tokens in %q", allowed, got, compressed)
	}
	if plan.Kind != RecoveryKindFullOutput || plan.Summary == "" || !plan.RequireRawCapture {
		t.Fatalf("expected recovery plan for compressed output, got %#v", plan)
	}
}

func TestEnforceCompressionContractBudgetsAgainstStreamedRawTokens(t *testing.T) {
	t.Parallel()

	// Simulate a preview-truncated capture: rawCombined only holds a small
	// prefix of the true output while the streamed token counter saw all of
	// it. The retained-token budget must follow the streamed count.
	preview := strings.Repeat("token ", 80)
	text := strings.Repeat("keep ", 60)
	streamedRawTokens := 1000

	compressed, plan, changed := enforceCompressionContract(text, preview, streamedRawTokens, OutputBudget{MaxTokens: 512}, RecoveryPlan{}, false, true)
	if changed {
		t.Fatalf("expected true-raw budget to keep filtered text, got %q (plan %#v)", compressed, plan)
	}
	if compressed != text {
		t.Fatalf("unexpected text change without compression: %q", compressed)
	}

	// The same preview without the streamed count must still compress, which
	// pins the old (buggy) preview-derived budget as the tighter one.
	previewCompressed, previewPlan, previewChanged := enforceCompressionContract(text, preview, 0, OutputBudget{MaxTokens: 512}, RecoveryPlan{}, false, true)
	if !previewChanged {
		t.Fatalf("expected preview-only budget to compress, got %q", previewCompressed)
	}
	if !strings.Contains(previewPlan.Summary, "from "+strconv.Itoa(history.EstimateTokens(preview))) {
		t.Fatalf("expected recovery summary to report raw token source, got %q", previewPlan.Summary)
	}
}

func TestCompressionRecoverySummaryReportsTrueRawTokens(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("word ", 300)
	text := strings.Repeat("keep ", 200)
	_, plan, changed := enforceCompressionContract(text, raw[:200], 300, OutputBudget{MaxTokens: 512}, RecoveryPlan{}, false, true)
	if !changed {
		t.Fatal("expected compression for oversized filtered text")
	}
	if !strings.Contains(plan.Summary, "from 300") {
		t.Fatalf("expected summary to report streamed raw tokens, got %q", plan.Summary)
	}
}

func TestHardCapTokensPreservesHighValueAnchors(t *testing.T) {
	t.Parallel()

	text := strings.Join([]string{
		"progress", "update", "noise", "noise", "noise", "noise", "noise", "noise",
		"panic", "src/api/handler.go:87", "undefined", "RenderWidget", "rerun", "with", "--debug",
		"inspect", "/tmp/widget.log", "after", "failure", "noise", "noise",
	}, " ")

	compressed := hardCapTokens(text, 18)
	if got := history.EstimateTokens(compressed); got > 18 {
		t.Fatalf("expected compressed output to respect token cap, got %d in %q", got, compressed)
	}
	for _, want := range []string{"panic", "src/api/handler.go:87", "RenderWidget", "rerun"} {
		if !strings.Contains(compressed, want) {
			t.Fatalf("expected compressed output to retain %q, got %q", want, compressed)
		}
	}
	if !strings.Contains(compressed, "...") {
		t.Fatalf("expected elision markers in compressed output, got %q", compressed)
	}
}

func TestEnforceCompressionContractSkipsSmallRawOutput(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 20)
	text := strings.Repeat("token ", 18)
	compressed, _, changed := enforceCompressionContract(text, raw, 0, OutputBudget{MaxTokens: 8}, RecoveryPlan{}, false, true)
	if changed {
		t.Fatal("did not expect contract on small raw output")
	}
	if compressed != text {
		t.Fatalf("unexpected small-output change: %q", compressed)
	}
}

func TestEnforceCompressionContractDisabled(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 80)
	text := strings.Repeat("token ", 40)
	compressed, _, changed := enforceCompressionContract(text, raw, 0, OutputBudget{MaxTokens: 16}, RecoveryPlan{}, false, false)
	if changed || compressed != text {
		t.Fatalf("expected disabled compression contract to preserve text, got changed=%v text=%q", changed, compressed)
	}
}

func TestPreferRawSmallOutput(t *testing.T) {
	t.Parallel()

	raw := "alpha\nbeta"
	rendered := "alpha beta [tee: 123456789012]"
	if got := preferRawSmallOutput(rendered, raw); got != raw {
		t.Fatalf("expected raw to win for small expanded output, got %q", got)
	}
	largeRaw := strings.Repeat("token ", 120)
	if got := preferRawSmallOutput("alpha", largeRaw); got == strings.TrimSpace(largeRaw) {
		t.Fatal("did not expect raw preference for large payloads")
	}
}
