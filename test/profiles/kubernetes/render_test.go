package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestKubectlProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	kubectlGet := testutil.FindProfile(t, list, "kubectl-get")
	rendered := kubectlGet.Render(engine.Invocation{}, engine.Execution{
		Stdout: `{"kind":"DeploymentList","items":[{"kind":"Deployment","metadata":{"name":"api","namespace":"default"},"status":{"replicas":3,"readyReplicas":2,"availableReplicas":2}}]}`,
	})
	for _, want := range []string{"deployment: 1 items", "deployment api ns=default replicas=2/3 available=2"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in kubectl get render output:\n%s", want, rendered)
		}
	}
	if kubectlGet.StreamPreference != engine.StreamStdoutOnly || kubectlGet.StreamRender == nil {
		t.Fatalf("unexpected kubectl get stream metadata: %#v", kubectlGet)
	}

	kubectlLogs := testutil.FindProfile(t, list, "kubectl-logs")
	streamed := kubectlLogs.StreamRender(engine.Invocation{}, kubectlLogs.Budget)
	streamed.ConsumeStdout([]byte("api-7d9/api ERROR failed to connect\n"))
	streamed.ConsumeStderr([]byte("worker-6f4/worker panic: queue broken\n"))
	got := streamed.Result()
	for _, want := range []string{"sources: 2", "api-7d9/api: ERROR failed to connect", "worker-6f4/worker: panic: queue broken"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in kubectl logs streamed output:\n%s", want, got)
		}
	}
	if kubectlLogs.StreamPreference != engine.StreamStdoutFirst || kubectlLogs.StreamRender == nil {
		t.Fatalf("unexpected kubectl logs stream metadata: %#v", kubectlLogs)
	}
}
