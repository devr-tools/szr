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
