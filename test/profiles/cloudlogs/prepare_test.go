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
	supabaseFunctions := testutil.FindProfile(t, list, "supabase-function-logs")
	herokuRouter := testutil.FindProfile(t, list, "heroku-router-logs")

	for _, display := range [][]string{
		{"aws", "logs", "tail", "/aws/lambda/api"},
		{"gcloud", "logging", "read", "severity>=ERROR"},
		{"az", "monitor", "activity-log", "list"},
		{"oci", "logging-search", "search-logs"},
		{"doctl", "apps", "logs", "app-123"},
		{"openstack", "console", "log", "show", "server-1"},
		{"vercel", "logs", "web-prod"},
		{"supabase", "logs"},
		{"heroku", "logs", "--app", "api-prod"},
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
	if got := profile.Prepare(engine.Invocation{Command: []string{"oci", "logging-search", "search-logs"}}); !reflect.DeepEqual(got, []string{"oci", "logging-search", "search-logs", "--output", "json"}) {
		t.Fatalf("unexpected oci log prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"doctl", "apps", "logs", "app-123"}}); !reflect.DeepEqual(got, []string{"doctl", "apps", "logs", "app-123"}) {
		t.Fatalf("expected doctl logs passthrough: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"supabase", "logs"}}); !reflect.DeepEqual(got, []string{"supabase", "logs", "--output", "json"}) {
		t.Fatalf("unexpected supabase logs prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"vercel", "logs", "web-prod"}}); !reflect.DeepEqual(got, []string{"vercel", "logs", "web-prod"}) {
		t.Fatalf("expected vercel logs passthrough: %#v", got)
	}
	if !supabaseFunctions.Match(engine.Invocation{Display: []string{"supabase", "functions", "logs", "stripe-webhook"}}) {
		t.Fatal("expected supabase functions logs to match specific profile")
	}
	if got := supabaseFunctions.Prepare(engine.Invocation{Command: []string{"supabase", "functions", "logs", "stripe-webhook"}}); !reflect.DeepEqual(got, []string{"supabase", "functions", "logs", "stripe-webhook", "--output", "json"}) {
		t.Fatalf("unexpected supabase function log prepare: %#v", got)
	}
	if !herokuRouter.Match(engine.Invocation{Display: []string{"heroku", "logs", "--app", "api-prod"}}) {
		t.Fatal("expected heroku logs to match router-specific profile")
	}
	if got := herokuRouter.Prepare(engine.Invocation{Command: []string{"heroku", "logs", "--app", "api-prod"}}); !reflect.DeepEqual(got, []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"}) {
		t.Fatalf("unexpected heroku router prepare: %#v", got)
	}
	if got := herokuRouter.Prepare(engine.Invocation{Command: []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"}}); !reflect.DeepEqual(got, []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"}) {
		t.Fatalf("expected explicit heroku source to be preserved: %#v", got)
	}
}
