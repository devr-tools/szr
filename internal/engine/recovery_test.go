package engine

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/history"
)

func TestReducerRecoveryPlanReadsProvider(t *testing.T) {
	reducer := &recoveryStubReducer{
		kind:              RecoveryKindFullOutput,
		summary:           "omitted 3 paths",
		requireRawCapture: true,
	}

	plan := reducerRecoveryPlan(reducer)
	if plan.Kind != RecoveryKindFullOutput || plan.Summary != "omitted 3 paths" || !plan.RequireRawCapture {
		t.Fatalf("unexpected recovery plan: %+v", plan)
	}
}

func TestAppendRecoveryHint(t *testing.T) {
	rendered := appendRecoveryHint("summary", RecoveryPlan{
		Kind:    RecoveryKindFullOutput,
		Summary: "omitted 2 commits",
	}, "/tmp/full.log", false)
	if rendered != "summary\n[recovery: omitted 2 commits; full output: /tmp/full.log]" {
		t.Fatalf("unexpected recovery hint: %q", rendered)
	}
}

func TestFinalizeRenderedDisplayRespectsCompressionContract(t *testing.T) {
	raw := strings.Repeat("token ", 80)
	rendered := strings.Repeat("token ", 40)
	budget := OutputBudget{MaxTokens: 16}

	final := finalizeRenderedDisplay(rendered, raw, budget, RecoveryPlan{
		Kind:    RecoveryKindFullOutput,
		Summary: "omitted many lines",
	}, "/tmp/full.log", false, true, true, false)

	allowed := compressionContractAllowedTokens(history.EstimateTokens(raw), budget)
	if got := history.EstimateTokens(final); got > allowed {
		t.Fatalf("expected final rendered display <= %d tokens, got %d (%q)", allowed, got, final)
	}
	if !strings.Contains(final, "[") {
		t.Fatalf("expected recovery or full-output suffix, got %q", final)
	}
}

func TestArtifactDisplayRefCompact(t *testing.T) {
	if got := artifactDisplayRef("/tmp/1234567890123456_command.log", true); got != "tee: 123456789012" {
		t.Fatalf("unexpected compact artifact ref: %q", got)
	}
}

type recoveryStubReducer struct {
	kind              string
	summary           string
	requireRawCapture bool
}

func (r *recoveryStubReducer) ConsumeStdout([]byte) {}

func (r *recoveryStubReducer) ConsumeStderr([]byte) {}

func (r *recoveryStubReducer) Result() string { return "" }

func (r *recoveryStubReducer) BytesParsed() int { return 0 }

func (r *recoveryStubReducer) FallbackUsed() bool { return false }

func (r *recoveryStubReducer) RecoveryInfo() (string, string, bool) {
	return r.kind, r.summary, r.requireRawCapture
}
