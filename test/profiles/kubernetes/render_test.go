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

func TestKubectlProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	kubectlGet := testutil.FindProfile(t, list, "kubectl-get")
	getStream := kubectlGet.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	getStream.ConsumeStdout([]byte(`{"kind":"DeploymentList","items":[{"kind":"Deployment","metadata":{"name":"api","namespace":"default"},"status":{"replicas":3,"readyReplicas":2,"availableReplicas":2}},{"kind":"Deployment","metadata":{"name":"worker","namespace":"jobs"},"status":{"replicas":1,"readyReplicas":0,"availableReplicas":0}},{"kind":"Deployment","metadata":{"name":"cron","namespace":"jobs"},"status":{"replicas":1,"readyReplicas":1,"availableReplicas":1}}]}`))
	getRecovery, ok := getStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable kubectl get reducer, got %T", getStream)
	}
	if kind, summary, requireRawCapture := getRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional resources" || !requireRawCapture {
		t.Fatalf("unexpected kubectl get recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	kubectlLogs := testutil.FindProfile(t, list, "kubectl-logs")
	logsStream := kubectlLogs.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	logsStream.ConsumeStdout([]byte("api-7d9/api ERROR failed to connect\napi-7d9/api WARN backing off\n"))
	logsStream.ConsumeStderr([]byte("worker-6f4/worker panic: queue broken\n"))
	logsRecovery, ok := logsStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable kubectl logs reducer, got %T", logsStream)
	}
	if kind, summary, requireRawCapture := logsRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional log lines" || !requireRawCapture {
		t.Fatalf("unexpected kubectl logs recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	kubectlTop := testutil.FindProfile(t, list, "kubectl-top")
	topStream := kubectlTop.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	topStream.ConsumeStdout([]byte(strings.Join([]string{
		"NAME CPU MEM",
		"api 10m 64Mi",
		"worker 200m 512Mi",
	}, "\n")))
	topRecovery, ok := topStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable kubectl top reducer, got %T", topStream)
	}
	if kind, summary, requireRawCapture := topRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional line" || !requireRawCapture {
		t.Fatalf("unexpected kubectl top recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
