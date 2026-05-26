package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	cloudprofiles "github.com/devr-tools/szr/internal/profiles/cloudlogs"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCloudLogsProfileRender(t *testing.T) {
	list := cloudprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "cloud-logs")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: "2026-05-25T10:00:00Z api ERROR timeout talking to redis\n2026-05-25T10:00:05Z api ERROR timeout talking to redis\n",
	})
	for _, want := range []string{
		"events: 2 sources: 1",
		"services: api",
		"api: ERROR timeout talking to redis (x2)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in cloud-logs render output:\n%s", want, rendered)
		}
	}

	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected cloud-logs stream metadata: %#v", profile)
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte(`[{"timestamp":"2026-05-25T10:00:00Z","severity":"ERROR","textPayload":"request failed","resource":{"type":"k8s_container","labels":{"container_name":"api"}}}]`))
	got := streamed.Result()
	for _, want := range []string{
		"events: 1 sources: 1",
		"services: k8s_container",
		"k8s_container: ERROR request failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cloud-logs streamed output:\n%s", want, got)
		}
	}
}
