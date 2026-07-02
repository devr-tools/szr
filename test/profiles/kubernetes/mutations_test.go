package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func firstMatchingProfileName(list []engine.Profile, display []string) string {
	for _, profile := range list {
		if profile.Match != nil && profile.Match(engine.Invocation{Display: display}) {
			return profile.Name
		}
	}
	return ""
}

func TestKubectlMutationProfilesMatch(t *testing.T) {
	list := profiles.Builtins(6)

	kubectlApply := testutil.FindProfile(t, list, "kubectl-apply")
	if !kubectlApply.Match(engine.Invocation{Display: []string{"kubectl", "apply", "-f", "manifests.yml"}}) {
		t.Fatal("expected kubectl apply to match")
	}
	if !kubectlApply.Match(engine.Invocation{Display: []string{"kubectl", "-n", "prod", "apply", "-f", "manifests.yml"}}) {
		t.Fatal("expected namespaced kubectl apply to match")
	}
	if !kubectlApply.Match(engine.Invocation{Display: []string{"kubectl", "delete", "-f", "manifests.yml"}}) {
		t.Fatal("expected kubectl delete to match")
	}
	if kubectlApply.Match(engine.Invocation{Display: []string{"kubectl", "get", "pods"}}) {
		t.Fatal("expected kubectl get to bypass kubectl-apply")
	}

	kubectlRollout := testutil.FindProfile(t, list, "kubectl-rollout")
	if !kubectlRollout.Match(engine.Invocation{Display: []string{"kubectl", "rollout", "status", "deployment/web"}}) {
		t.Fatal("expected kubectl rollout status to match")
	}

	kubectlDiff := testutil.FindProfile(t, list, "kubectl-diff")
	if !kubectlDiff.Match(engine.Invocation{Display: []string{"kubectl", "diff", "-f", "manifests.yml"}}) {
		t.Fatal("expected kubectl diff to match")
	}

	// Ordering: the new matchers must not be shadowed by earlier profiles,
	// and must not shadow the existing kubectl read-verb profiles.
	for _, tc := range []struct {
		display []string
		want    string
	}{
		{[]string{"kubectl", "apply", "-f", "manifests.yml"}, "kubectl-apply"},
		{[]string{"kubectl", "delete", "pod", "api-1"}, "kubectl-apply"},
		{[]string{"kubectl", "rollout", "status", "deployment/web"}, "kubectl-rollout"},
		{[]string{"kubectl", "diff", "-f", "manifests.yml"}, "kubectl-diff"},
		{[]string{"kubectl", "get", "pods"}, "kubectl-get"},
		{[]string{"kubectl", "logs", "api"}, "kubectl-logs"},
	} {
		if got := firstMatchingProfileName(list, tc.display); got != tc.want {
			t.Fatalf("expected %v to first-match %q, got %q", tc.display, tc.want, got)
		}
	}
}

func TestKubectlMutationProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	kubectlApply := testutil.FindProfile(t, list, "kubectl-apply")
	rendered := kubectlApply.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"deployment.apps/api configured",
			"service/api unchanged",
			"namespace/staging created",
		}, "\n"),
		Stderr: "Warning: annotation missing",
	})
	for _, want := range []string{
		"resources: configured=1 unchanged=1 created=1",
		"configured: deployment.apps/api",
		"Warning: annotation missing",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in kubectl apply render output:\n%s", want, rendered)
		}
	}
	if kubectlApply.StreamPreference != engine.StreamStdoutFirst || kubectlApply.StreamRender == nil {
		t.Fatalf("unexpected kubectl apply stream metadata: %#v", kubectlApply)
	}

	kubectlRollout := testutil.FindProfile(t, list, "kubectl-rollout")
	streamed := kubectlRollout.StreamRender(engine.Invocation{}, kubectlRollout.Budget)
	streamed.ConsumeStdout([]byte(strings.Join([]string{
		`Waiting for deployment "web" rollout to finish: 1 of 3 updated replicas are available...`,
		`Waiting for deployment "web" rollout to finish: 2 of 3 updated replicas are available...`,
		`deployment "web" successfully rolled out`,
	}, "\n")))
	got := streamed.Result()
	for _, want := range []string{
		"progress: collapsed 1 rollout updates",
		`deployment "web" successfully rolled out`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl rollout streamed output:\n%s", want, got)
		}
	}

	kubectlDiff := testutil.FindProfile(t, list, "kubectl-diff")
	rendered = kubectlDiff.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"diff -u -N /tmp/LIVE-123/apps.v1.Deployment.default.web /tmp/MERGED-456/apps.v1.Deployment.default.web",
			"--- /tmp/LIVE-123/apps.v1.Deployment.default.web",
			"+++ /tmp/MERGED-456/apps.v1.Deployment.default.web",
			"-  replicas: 2",
			"+  replicas: 3",
		}, "\n"),
	})
	for _, want := range []string{"diff: 1 objects changed", "apps.v1.Deployment.default.web: +1 -1"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in kubectl diff render output:\n%s", want, rendered)
		}
	}
}
