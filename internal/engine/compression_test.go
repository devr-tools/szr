package engine

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
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
	compressed, plan, changed := enforceCompressionContract(text, raw, OutputBudget{MaxTokens: 32}, RecoveryPlan{}, false, true)
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

func TestEnforceCompressionContractSkipsSmallRawOutput(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 20)
	text := strings.Repeat("token ", 18)
	compressed, _, changed := enforceCompressionContract(text, raw, OutputBudget{MaxTokens: 8}, RecoveryPlan{}, false, true)
	if changed {
		t.Fatal("did not expect contract on small raw output")
	}
	if compressed != text {
		t.Fatalf("unexpected small-output change: %q", compressed)
	}
}

func TestRenderExecutionAppliesCompressionContract(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 80)
	profile := Profile{
		Name:   "contract-render",
		Budget: OutputBudget{MaxLines: 12, MaxTokens: 32},
		Render: func(_ Invocation, exec Execution) string {
			return exec.Stdout
		},
	}
	rendered := RenderExecution(profile, Invocation{Advanced: configAdvancedForTests()}, Execution{Stdout: raw}, 12, false)
	allowed := compressionContractAllowedTokens(rendered.RawTokens, profile.Budget)
	if rendered.FilteredTokens > allowed {
		t.Fatalf("expected rendered tokens <= %d, got %d (%q)", allowed, rendered.FilteredTokens, rendered.Text)
	}
}

func TestEnforceCompressionContractDisabled(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("token ", 80)
	text := strings.Repeat("token ", 40)
	compressed, _, changed := enforceCompressionContract(text, raw, OutputBudget{MaxTokens: 16}, RecoveryPlan{}, false, false)
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

func configAdvancedForTests() config.Advanced {
	return config.Advanced{CompressionContract: true, CompactArtifactRefs: true}
}
