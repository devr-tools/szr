package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	cloudprofiles "github.com/devr-tools/szr/internal/profiles/cloudlogs"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCloudLogsProfilePrepare(t *testing.T) {
	list := cloudprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "cloud-logs")

	for _, display := range [][]string{
		{"aws", "logs", "tail", "/aws/lambda/api"},
		{"gcloud", "logging", "read", "severity>=ERROR"},
		{"az", "monitor", "activity-log", "list"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match cloud-logs", display)
		}
	}
	if profile.Match(engine.Invocation{Display: []string{"aws", "ec2", "describe-instances"}}) {
		t.Fatal("did not expect non-log cloud command to match")
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"aws", "logs", "tail", "/aws/lambda/api"}}); !reflect.DeepEqual(got, []string{"aws", "logs", "tail", "/aws/lambda/api"}) {
		t.Fatalf("expected aws logs tail passthrough: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"gcloud", "logging", "read", "severity>=ERROR"}}); !reflect.DeepEqual(got, []string{"gcloud", "logging", "read", "severity>=ERROR", "--format=json"}) {
		t.Fatalf("unexpected gcloud logging prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"az", "monitor", "activity-log", "list"}}); !reflect.DeepEqual(got, []string{"az", "monitor", "activity-log", "list", "-o", "json"}) {
		t.Fatalf("unexpected az monitor prepare: %#v", got)
	}
}
