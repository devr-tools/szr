package engine

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/history"
)

func TestCompressionContractAllowedTokens(t *testing.T) {
	t.Parallel()

	// Large outputs retain <=1/5 of the raw tokens.
	if got := compressionContractAllowedTokens(1000, OutputBudget{MaxTokens: 500}); got != 200 {
		t.Fatalf("expected 200 retained tokens, got %d", got)
	}
	if got := compressionContractAllowedTokens(1000, OutputBudget{MaxTokens: 100}); got != 100 {
		t.Fatalf("expected budget cap to win, got %d", got)
	}
	// The fidelity floor is applied last: neither the 1/5 rule nor a tiny
	// profile budget may crush a render below a usable size.
	if got := compressionContractAllowedTokens(200, OutputBudget{}); got != compressionContractMinTokens {
		t.Fatalf("expected minimum token floor, got %d", got)
	}
	if got := compressionContractAllowedTokens(1000, OutputBudget{MaxTokens: 6}); got != compressionContractMinTokens {
		t.Fatalf("expected floor to beat tiny budget, got %d", got)
	}
}

func TestEnforceCompressionContractCompressesLargeOutput(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 400)
	text := strings.Repeat("token ", 200)
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
	preview := strings.Repeat("token ", 200)
	text := strings.Repeat("keep ", 100)
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

// TestEnforceCompressionContractLeavesSmallDiagnosticsIntact reproduces a
// benchmark loss: golangci-lint reported two concrete issues in ~61 raw
// tokens and the old 40-token contract threshold crushed the render to a
// bare "..." — 24 bytes of zero signal on a failing command. Small outputs
// must pass through untouched; the fidelity floor only lets the contract
// bite where absolute savings matter.
func TestEnforceCompressionContractLeavesSmallDiagnosticsIntact(t *testing.T) {
	t.Parallel()

	raw := strings.Join([]string{
		"calc/calc.go:20:12: Error return value of `fmt.Errorf` is not checked (errcheck)",
		"calc/calc.go:17:6: func unusedHelper is unused (unused)",
		"2 issues:",
		"* errcheck: 1",
		"* unused: 1",
	}, "\n")
	rawTokens := history.EstimateTokens(raw)
	if rawTokens < 40 || rawTokens >= compressionContractMinRawTokens {
		t.Fatalf("fixture must sit between the old and new thresholds, got %d tokens", rawTokens)
	}

	compressed, _, changed := enforceCompressionContract(raw, raw, rawTokens, OutputBudget{MaxTokens: 13}, RecoveryPlan{}, false, true)
	if changed {
		t.Fatalf("expected contract to skip small diagnostic output, got %q", compressed)
	}
	for _, want := range []string{
		"calc/calc.go:20:12: Error return value of `fmt.Errorf` is not checked (errcheck)",
		"calc/calc.go:17:6: func unusedHelper is unused (unused)",
	} {
		if !strings.Contains(compressed, want) {
			t.Fatalf("expected issue line %q verbatim, got %q", want, compressed)
		}
	}
}

// TestHardCapTokensKeepsWholeLinesFirst pins the line-first behavior: when a
// multi-line render exceeds the cap, whole high-value lines survive verbatim
// instead of being shredded into word salad.
func TestHardCapTokensKeepsWholeLinesFirst(t *testing.T) {
	t.Parallel()

	issueOne := "calc/calc.go:20:12: Error return value of `fmt.Errorf` is not checked (errcheck)"
	issueTwo := "calc/calc.go:17:6: func unusedHelper is unused (unused)"
	lines := []string{issueOne, issueTwo}
	for i := 0; i < 30; i++ {
		lines = append(lines, "level=info msg=progress step accepted queue drained without incident")
	}
	compressed := hardCapTokens(strings.Join(lines, "\n"), 48)
	if got := history.EstimateTokens(compressed); got > 48 {
		t.Fatalf("expected capped output within 48 tokens, got %d in %q", got, compressed)
	}
	for _, want := range []string{issueOne, issueTwo} {
		if !strings.Contains(compressed, want) {
			t.Fatalf("expected whole issue line %q to survive, got %q", want, compressed)
		}
	}
	for _, line := range strings.Split(compressed, "\n") {
		if line != "..." && line != issueOne && line != issueTwo && line != lines[2] {
			t.Fatalf("expected only whole input lines or markers, got fragment %q in %q", line, compressed)
		}
	}
}

// TestHardCapTokensKeepsHeadlineFirstLine pins the headline guarantee: the
// first content line of a render is its summary line by convention across
// szr profiles, so capping must never let path-dense detail lines crowd it
// out (the gh-pr-view regression: file-churn lines survived while the PR
// title line was dropped).
func TestHardCapTokensKeepsHeadlineFirstLine(t *testing.T) {
	t.Parallel()

	headline := "#45 OPEN feat: summarize raw gh api JSON responses [review:none] feat-gh-api->main"
	lines := []string{headline}
	for i := 0; i < 24; i++ {
		lines = append(lines, "internal/cli/spread.go +15 -2")
		lines = append(lines, "internal/history/summary.go +7 -2")
	}
	compressed := hardCapTokens(strings.Join(lines, "\n"), 48)
	if got := history.EstimateTokens(compressed); got > 48 {
		t.Fatalf("expected capped output within 48 tokens, got %d in %q", got, compressed)
	}
	if !strings.HasPrefix(compressed, headline) {
		t.Fatalf("expected headline to survive capping as the first line, got %q", compressed)
	}
}

// TestHardCapTokensNeverContentFree pins the fidelity floor of the cap
// itself: even an absurdly small cap keeps the single highest-value line
// rather than rendering nothing but ellipses.
func TestHardCapTokensNeverContentFree(t *testing.T) {
	t.Parallel()

	text := strings.Join([]string{
		"noise noise noise noise noise noise noise noise noise noise",
		"error: connection refused dial tcp localhost:8080",
		"noise noise noise noise noise noise noise noise noise noise",
	}, "\n")
	for _, capTokens := range []int{1, 2, 4, 8} {
		compressed := hardCapTokens(text, capTokens)
		if isContentFreeRender(compressed) {
			t.Fatalf("expected content at cap %d, got %q", capTokens, compressed)
		}
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
