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
