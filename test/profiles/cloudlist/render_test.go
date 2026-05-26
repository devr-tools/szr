package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	cloudprofiles "github.com/devr-tools/szr/internal/profiles/cloudlist"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCloudListProfileRenderAndStream(t *testing.T) {
	list := cloudprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "cloud-list")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: `[{"name":"api-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-a","status":"RUNNING","project":"demo-project"}]`,
	})
	for _, want := range []string{"resources: 1", "api-prod zone=us-central1-a status=RUNNING project=demo-project"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in cloud-list render output:\n%s", want, rendered)
		}
	}

	if profile.StreamPreference != engine.StreamStdoutOnly || profile.StreamRender == nil {
		t.Fatalf("unexpected cloud-list stream metadata: %#v", profile)
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte(`{"Reservations":[{"Instances":[{"InstanceId":"i-123","Tags":[{"Key":"Name","Value":"api"}],"Placement":{"AvailabilityZone":"us-east-1a"},"State":{"Name":"running"}}]}]}`))
	got := streamed.Result()
	for _, want := range []string{"instances: 1", "api", "zone=us-east-1a", "status=running"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cloud-list streamed output:\n%s", want, got)
		}
	}
}
