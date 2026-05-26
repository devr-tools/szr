package engine

import "testing"

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
