package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestDockerProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	dockerPS := testutil.FindProfile(t, list, "docker-ps")
	rendered := dockerPS.Render(engine.Invocation{}, engine.Execution{
		Stdout: "api\tUp 2 minutes\tapp:latest\nworker\tExited (1) 4 seconds ago\tworker:latest\n",
	})
	for _, want := range []string{"containers: running=1 exited=1 other=0", "api: Up 2 minutes [app:latest]"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in docker ps render output:\n%s", want, rendered)
		}
	}
	if dockerPS.StreamPreference != engine.StreamStdoutOnly || dockerPS.StreamRender == nil {
		t.Fatalf("unexpected docker ps stream metadata: %#v", dockerPS)
	}

	dockerLogs := testutil.FindProfile(t, list, "docker-logs")
	streamed := dockerLogs.StreamRender(engine.Invocation{}, dockerLogs.Budget)
	streamed.ConsumeStdout([]byte("api-1  | ERROR failed to connect\n"))
	streamed.ConsumeStderr([]byte("worker-1  | panic: queue broken\n"))
	got := streamed.Result()
	for _, want := range []string{"sources: 2", "api-1: ERROR failed to connect", "worker-1: panic: queue broken"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in docker logs streamed output:\n%s", want, got)
		}
	}
	if dockerLogs.StreamPreference != engine.StreamStdoutFirst || dockerLogs.StreamRender == nil {
		t.Fatalf("unexpected docker logs stream metadata: %#v", dockerLogs)
	}
}

func TestDockerProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	dockerPS := testutil.FindProfile(t, list, "docker-ps")
	psStream := dockerPS.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 2})
	psStream.ConsumeStdout([]byte(strings.Join([]string{
		"api\tUp 2 minutes\tapp:latest",
		"worker\tExited (1) 4 seconds ago\tworker:latest",
		"cron\tUp 1 minute\tcron:latest",
	}, "\n")))
	psRecovery, ok := psStream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable docker ps reducer, got %T", psStream)
	}
	if kind, summary, requireRawCapture := psRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional containers" || !requireRawCapture {
		t.Fatalf("unexpected docker ps recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	dockerLogs := testutil.FindProfile(t, list, "docker-logs")
	logsStream := dockerLogs.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	logsStream.ConsumeStdout([]byte(strings.Join([]string{
		"api-1  | ERROR failed to connect",
		"api-1  | WARN backing off",
		"worker-1  | panic: queue broken",
		"worker-1  | fatal: exiting",
	}, "\n")))
	logsRecovery, ok := logsStream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable docker logs reducer, got %T", logsStream)
	}
	if kind, summary, requireRawCapture := logsRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional log lines" || !requireRawCapture {
		t.Fatalf("unexpected docker logs recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
