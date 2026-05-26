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

	if !profile.Match(engine.Invocation{Display: []string{"aws", "ec2", "describe-instances"}}) {
		t.Fatal("expected aws describe command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"aws", "ec2", "describe-instances"}}); !reflect.DeepEqual(got, []string{"aws", "ec2", "describe-instances", "--output", "json"}) {
		t.Fatalf("unexpected aws prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"aws", "s3api", "list-buckets", "--output", "table"}}); !reflect.DeepEqual(got, []string{"aws", "s3api", "list-buckets", "--output", "table"}) {
		t.Fatalf("expected explicit aws output to be preserved: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"gcloud", "--project", "demo", "compute", "instances", "list"}}) {
		t.Fatal("expected gcloud list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"gcloud", "compute", "instances", "list"}}); !reflect.DeepEqual(got, []string{"gcloud", "compute", "instances", "list", "--format=json"}) {
		t.Fatalf("unexpected gcloud prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"gcloud", "projects", "describe", "demo", "--format=yaml"}}); !reflect.DeepEqual(got, []string{"gcloud", "projects", "describe", "demo", "--format=yaml"}) {
		t.Fatalf("expected explicit gcloud format to be preserved: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"az", "--subscription", "prod", "vm", "list"}}) {
		t.Fatal("expected az list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"az", "group", "show", "-n", "prod-rg"}}); !reflect.DeepEqual(got, []string{"az", "group", "show", "-n", "prod-rg", "-o", "json"}) {
		t.Fatalf("unexpected az prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"az", "vm", "list", "-o", "table"}}); !reflect.DeepEqual(got, []string{"az", "vm", "list", "-o", "table"}) {
		t.Fatalf("expected explicit az output to be preserved: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"doctl", "compute", "droplet", "list"}}) {
		t.Fatal("expected doctl list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"doctl", "compute", "droplet", "list"}}); !reflect.DeepEqual(got, []string{"doctl", "compute", "droplet", "list", "--output", "json"}) {
		t.Fatalf("unexpected doctl prepare: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"oci", "compute", "instance", "list"}}) {
		t.Fatal("expected oci list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"oci", "compute", "instance", "list"}}); !reflect.DeepEqual(got, []string{"oci", "compute", "instance", "list", "--output", "json"}) {
		t.Fatalf("unexpected oci prepare: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"openstack", "server", "list"}}) {
		t.Fatal("expected openstack list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"openstack", "server", "show", "api-1"}}); !reflect.DeepEqual(got, []string{"openstack", "server", "show", "api-1", "-f", "json"}) {
		t.Fatalf("unexpected openstack prepare: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"vercel", "projects", "ls"}}) {
		t.Fatal("expected vercel projects list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"vercel", "projects", "ls"}}); !reflect.DeepEqual(got, []string{"vercel", "projects", "ls", "--json"}) {
		t.Fatalf("unexpected vercel prepare: %#v", got)
	}
	if !vercelDeployments.Match(engine.Invocation{Display: []string{"vercel", "ls"}}) {
		t.Fatal("expected vercel deployments list to match specific profile")
	}
	if got := vercelDeployments.Prepare(engine.Invocation{Command: []string{"vercel", "ls"}}); !reflect.DeepEqual(got, []string{"vercel", "ls", "--json", "--meta"}) {
		t.Fatalf("unexpected vercel deployment prepare: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"supabase", "projects", "list"}}) {
		t.Fatal("expected supabase projects list command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"supabase", "projects", "list"}}); !reflect.DeepEqual(got, []string{"supabase", "projects", "list", "--output", "json"}) {
		t.Fatalf("unexpected supabase prepare: %#v", got)
	}

	if !profile.Match(engine.Invocation{Display: []string{"heroku", "apps"}}) {
		t.Fatal("expected heroku apps command to match")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"heroku", "apps"}}); !reflect.DeepEqual(got, []string{"heroku", "apps", "--json"}) {
		t.Fatalf("unexpected heroku prepare: %#v", got)
	}

	if profile.Match(engine.Invocation{Display: []string{"aws", "s3", "cp", "a", "b"}}) {
		t.Fatal("did not expect non-inventory cloud command to match")
	}
	if len(profile.Explain) != 2 {
		t.Fatalf("expected explain lines, got %#v", profile.Explain)
	}
}
