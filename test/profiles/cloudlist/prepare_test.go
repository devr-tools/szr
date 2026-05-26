package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	cloudprofiles "github.com/devr-tools/szr/internal/profiles/cloudlist"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCloudListProfilePrepare(t *testing.T) {
	list := cloudprofiles.Profiles(6)
	profile := testutil.FindProfile(t, list, "cloud-list")
	vercelDeployments := testutil.FindProfile(t, list, "vercel-deployments")

	for _, display := range []engine.Invocation{
		{Display: []string{"aws", "ec2", "describe-instances"}},
		{Display: []string{"gcloud", "--project", "demo", "compute", "instances", "list"}},
		{Display: []string{"az", "--subscription", "prod", "vm", "list"}},
		{Display: []string{"doctl", "compute", "droplet", "list"}},
		{Display: []string{"oci", "compute", "instance", "list"}},
		{Display: []string{"openstack", "server", "list"}},
		{Display: []string{"vercel", "projects", "ls"}},
		{Display: []string{"supabase", "projects", "list"}},
		{Display: []string{"heroku", "apps"}},
	} {
		if !profile.Match(display) {
			t.Fatalf("expected %#v to match cloud-list", display.Display)
		}
	}

	for _, tc := range []struct {
		name string
		got  []string
		want []string
	}{
		{name: "aws", got: profile.Prepare(engine.Invocation{Command: []string{"aws", "ec2", "describe-instances"}}), want: []string{"aws", "ec2", "describe-instances", "--output", "json"}},
		{name: "aws preserve output", got: profile.Prepare(engine.Invocation{Command: []string{"aws", "s3api", "list-buckets", "--output", "table"}}), want: []string{"aws", "s3api", "list-buckets", "--output", "table"}},
		{name: "gcloud", got: profile.Prepare(engine.Invocation{Command: []string{"gcloud", "compute", "instances", "list"}}), want: []string{"gcloud", "compute", "instances", "list", "--format=json"}},
		{name: "gcloud preserve format", got: profile.Prepare(engine.Invocation{Command: []string{"gcloud", "projects", "describe", "demo", "--format=yaml"}}), want: []string{"gcloud", "projects", "describe", "demo", "--format=yaml"}},
		{name: "az", got: profile.Prepare(engine.Invocation{Command: []string{"az", "group", "show", "-n", "prod-rg"}}), want: []string{"az", "group", "show", "-n", "prod-rg", "-o", "json"}},
		{name: "az preserve output", got: profile.Prepare(engine.Invocation{Command: []string{"az", "vm", "list", "-o", "table"}}), want: []string{"az", "vm", "list", "-o", "table"}},
		{name: "doctl", got: profile.Prepare(engine.Invocation{Command: []string{"doctl", "compute", "droplet", "list"}}), want: []string{"doctl", "compute", "droplet", "list", "--output", "json"}},
		{name: "oci", got: profile.Prepare(engine.Invocation{Command: []string{"oci", "compute", "instance", "list"}}), want: []string{"oci", "compute", "instance", "list", "--output", "json"}},
		{name: "openstack", got: profile.Prepare(engine.Invocation{Command: []string{"openstack", "server", "show", "api-1"}}), want: []string{"openstack", "server", "show", "api-1", "-f", "json"}},
		{name: "vercel", got: profile.Prepare(engine.Invocation{Command: []string{"vercel", "projects", "ls"}}), want: []string{"vercel", "projects", "ls", "--json"}},
		{name: "supabase", got: profile.Prepare(engine.Invocation{Command: []string{"supabase", "projects", "list"}}), want: []string{"supabase", "projects", "list", "--output", "json"}},
		{name: "heroku", got: profile.Prepare(engine.Invocation{Command: []string{"heroku", "apps"}}), want: []string{"heroku", "apps", "--json"}},
	} {
		assertPreparedCloudCommand(t, tc.name, tc.got, tc.want)
	}
	if !vercelDeployments.Match(engine.Invocation{Display: []string{"vercel", "ls"}}) {
		t.Fatal("expected vercel deployments list to match specific profile")
	}
	assertPreparedCloudCommand(t, "vercel deployments", vercelDeployments.Prepare(engine.Invocation{Command: []string{"vercel", "ls"}}), []string{"vercel", "ls", "--json", "--meta"})

	if profile.Match(engine.Invocation{Display: []string{"aws", "s3", "cp", "a", "b"}}) {
		t.Fatal("did not expect non-inventory cloud command to match")
	}
	if len(profile.Explain) != 2 {
		t.Fatalf("expected explain lines, got %#v", profile.Explain)
	}
}

func assertPreparedCloudCommand(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected %s prepare: %#v", name, got)
	}
}
