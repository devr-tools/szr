package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestGHProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	ghPR := testutil.FindProfile(t, list, "gh-pr-view")
	rendered := ghPR.Render(engine.Invocation{}, engine.Execution{
		Stdout: `{"number":7,"title":"Add semantic kubectl reducers","state":"OPEN","isDraft":true,"headRefName":"feature/kube","baseRefName":"main","reviewDecision":"REVIEW_REQUIRED","files":[{"path":"internal/filters/kubernetes.go","additions":120,"deletions":0}]}`,
	})
	for _, want := range []string{"PR #7 Add semantic kubectl reducers state=open draft=true", "feature/kube -> main review=review_required", "internal/filters/kubernetes.go +120 -0"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in gh pr render output:\n%s", want, rendered)
		}
	}
	if ghPR.StreamPreference != engine.StreamStdoutOnly || ghPR.StreamRender == nil {
		t.Fatalf("unexpected gh pr stream metadata: %#v", ghPR)
	}

	ghRun := testutil.FindProfile(t, list, "gh-run-view")
	streamed := ghRun.StreamRender(engine.Invocation{}, ghRun.Budget)
	streamed.ConsumeStdout([]byte(`{"workflowName":"CI","status":"completed","conclusion":"failure","event":"pull_request","headBranch":"feature/kube","jobs":[{"name":"test","status":"completed","conclusion":"failure","steps":[{"name":"unit","conclusion":"failure"}]}]}`))
	got := streamed.Result()
	for _, want := range []string{"CI status=completed conclusion=failure", "branch=feature/kube event=pull_request", "job test status=completed conclusion=failure", "step unit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh run streamed output:\n%s", want, got)
		}
	}
	if ghRun.StreamPreference != engine.StreamStdoutFirst || ghRun.StreamRender == nil {
		t.Fatalf("unexpected gh run stream metadata: %#v", ghRun)
	}

	ghRunLog := testutil.FindProfile(t, list, "gh-run-log")
	logged := ghRunLog.Render(engine.Invocation{}, engine.Execution{
		Stdout: "test\tUnit\tError: assertion failed\ntest\tUnit\tError: assertion failed\n",
	})
	if !strings.Contains(logged, "test: Unit Error: assertion failed (x2)") {
		t.Fatalf("expected grouped gh run log output, got:\n%s", logged)
	}
	if ghRunLog.StreamPreference != engine.StreamStdoutFirst || ghRunLog.StreamRender == nil {
		t.Fatalf("unexpected gh run log stream metadata: %#v", ghRunLog)
	}
}

func TestGHProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	ghPR := testutil.FindProfile(t, list, "gh-pr-view")
	prStream := ghPR.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 4})
	prStream.ConsumeStdout([]byte(`{"number":7,"title":"Add semantic kubectl reducers","state":"OPEN","isDraft":true,"headRefName":"feature/kube","baseRefName":"main","reviewDecision":"REVIEW_REQUIRED","files":[{"path":"internal/filters/kubernetes.go","additions":120,"deletions":0},{"path":"internal/profiles/kubernetes.go","additions":30,"deletions":0},{"path":"test/kubernetes_test.go","additions":40,"deletions":2}]}`))
	prRecovery, ok := prStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable gh pr reducer, got %T", prStream)
	}
	if kind, summary, requireRawCapture := prRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh pr recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	ghRun := testutil.FindProfile(t, list, "gh-run-view")
	runStream := ghRun.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 4})
	runStream.ConsumeStdout([]byte(`{"workflowName":"CI","status":"completed","conclusion":"failure","event":"pull_request","headBranch":"feature/kube","jobs":[{"name":"test","status":"completed","conclusion":"failure","steps":[{"name":"unit","conclusion":"failure"},{"name":"integration","conclusion":"failure"}]},{"name":"lint","status":"completed","conclusion":"failure","steps":[{"name":"eslint","conclusion":"failure"}]}]}`))
	runRecovery, ok := runStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable gh run reducer, got %T", runStream)
	}
	if kind, summary, requireRawCapture := runRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 3 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh run recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	ghRunList := testutil.FindProfile(t, list, "gh-run-list")
	runListStream := ghRunList.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	runListStream.ConsumeStdout([]byte(strings.Join([]string{
		"completed  success  CI         main",
		"completed  failure  CI         feature/kube",
		"in_progress pending Deploy     main",
	}, "\n")))
	runListRecovery, ok := runListStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable gh run list reducer, got %T", runListStream)
	}
	if kind, summary, requireRawCapture := runListRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional line" || !requireRawCapture {
		t.Fatalf("unexpected gh run list recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
