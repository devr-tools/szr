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

	for _, tc := range []struct {
		name string
		got  []string
		want []string
	}{
		{name: "aws", got: profile.Prepare(engine.Invocation{Command: []string{"aws", "logs", "tail", "/aws/lambda/api"}}), want: []string{"aws", "logs", "tail", "/aws/lambda/api"}},
		{name: "gcloud", got: profile.Prepare(engine.Invocation{Command: []string{"gcloud", "logging", "read", "severity>=ERROR"}}), want: []string{"gcloud", "logging", "read", "severity>=ERROR", "--format=json"}},
		{name: "az", got: profile.Prepare(engine.Invocation{Command: []string{"az", "monitor", "activity-log", "list"}}), want: []string{"az", "monitor", "activity-log", "list", "-o", "json"}},
		{name: "oci", got: profile.Prepare(engine.Invocation{Command: []string{"oci", "logging-search", "search-logs"}}), want: []string{"oci", "logging-search", "search-logs", "--output", "json"}},
		{name: "doctl", got: profile.Prepare(engine.Invocation{Command: []string{"doctl", "apps", "logs", "app-123"}}), want: []string{"doctl", "apps", "logs", "app-123"}},
		{name: "supabase", got: profile.Prepare(engine.Invocation{Command: []string{"supabase", "logs"}}), want: []string{"supabase", "logs", "--output", "json"}},
		{name: "vercel", got: profile.Prepare(engine.Invocation{Command: []string{"vercel", "logs", "web-prod"}}), want: []string{"vercel", "logs", "web-prod"}},
	} {
		assertPreparedCommand(t, tc.name, tc.got, tc.want)
	}
	if !supabaseFunctions.Match(engine.Invocation{Display: []string{"supabase", "functions", "logs", "stripe-webhook"}}) {
		t.Fatal("expected supabase functions logs to match specific profile")
	}
	assertPreparedCommand(t, "supabase functions", supabaseFunctions.Prepare(engine.Invocation{Command: []string{"supabase", "functions", "logs", "stripe-webhook"}}), []string{"supabase", "functions", "logs", "stripe-webhook", "--output", "json"})
	if !herokuRouter.Match(engine.Invocation{Display: []string{"heroku", "logs", "--app", "api-prod"}}) {
		t.Fatal("expected heroku logs to match router-specific profile")
	}
	assertPreparedCommand(t, "heroku router", herokuRouter.Prepare(engine.Invocation{Command: []string{"heroku", "logs", "--app", "api-prod"}}), []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"})
	assertPreparedCommand(t, "heroku router preserve source", herokuRouter.Prepare(engine.Invocation{Command: []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"}}), []string{"heroku", "logs", "--app", "api-prod", "--source", "heroku"})
}

func assertPreparedCommand(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected %s prepare: %#v", name, got)
	}
}
