package cloudlist_test

import (
	"strings"
	"testing"

	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlist"
)

func TestSummarizeCloudListAWSInstances(t *testing.T) {
	input := `{"Reservations":[{"Instances":[{"InstanceId":"i-1234567890","Tags":[{"Key":"Name","Value":"api"}],"Placement":{"AvailabilityZone":"us-east-1a"},"State":{"Name":"running"},"PublicIpAddress":"54.1.2.3"}]}]}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"instances: 1",
		"api",
		"zone=us-east-1a",
		"status=running",
		"ip=54.1.2.3",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in AWS cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListGCloudResources(t *testing.T) {
	input := `[{"name":"api-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-a","status":"RUNNING","project":"demo-project"},{"name":"jobs-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-b","status":"TERMINATED","project":"demo-project"}]`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"resources: 2",
		"api-prod zone=us-central1-a status=RUNNING project=demo-project",
		"jobs-prod zone=us-central1-b status=TERMINATED project=demo-project",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gcloud cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListAzureResource(t *testing.T) {
	input := `{"name":"prod-rg","id":"/subscriptions/000/resourceGroups/prod-rg","location":"eastus","properties":{"provisioningState":"Succeeded"}}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"resource: 1",
		"prod-rg",
		"location=eastus",
		"status=Succeeded",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Azure cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListDigitalOceanDroplet(t *testing.T) {
	input := `{"droplets":[{"id":123,"name":"api-1","region_slug":"nyc3","status":"active","created_at":"2026-05-25T10:00:00Z"}]}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"droplets: 1",
		"api-1",
		"region=nyc3",
		"status=active",
		"created=2026-05-25T10:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in DigitalOcean cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListOCIInstance(t *testing.T) {
	input := `{"data":[{"id":"ocid1.instance.oc1..abc","displayName":"api-oci","availability_domain":"AD-1","lifecycleState":"RUNNING","time_created":"2026-05-25T10:00:00Z"}]}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"resources: 1",
		"api-oci",
		"zone=AD-1",
		"status=RUNNING",
		"created=2026-05-25T10:00:00Z",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in OCI cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListVercelProject(t *testing.T) {
	input := `[{"id":"prj_123","name":"web-prod","updated_at":"2026-05-25T10:00:00Z","framework":"nextjs"}]`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"resources: 1",
		"web-prod",
		"id=prj_123",
		"framework=nextjs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Vercel cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListVercelDeployment(t *testing.T) {
	input := `[{"uid":"dpl_123","name":"web","url":"web-git-main-acme.vercel.app","target":"production","readyState":"READY","creator":{"username":"alex"}}]`
	got := cloudfilter.SummarizeCloudList(input, 5)
	for _, want := range []string{
		"resources: 1",
		"web",
		"target=production",
		"state=READY",
		"url=web-git-main-acme.vercel.app",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Vercel deployment summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListSupabaseProject(t *testing.T) {
	input := `{"projects":[{"id":"sb_123","name":"db-prod","region":"us-east-1","status":"ACTIVE"}]}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"projects: 1",
		"db-prod",
		"region=us-east-1",
		"status=ACTIVE",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Supabase cloud list summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListSupabaseFunction(t *testing.T) {
	input := `{"functions":[{"name":"stripe-webhook","version":"12","verify_jwt":true,"status":"ACTIVE"}]}`
	got := cloudfilter.SummarizeCloudList(input, 4)
	for _, want := range []string{
		"functions: 1",
		"function stripe-webhook",
		"version=12",
		"verify_jwt=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Supabase function summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudListHerokuApp(t *testing.T) {
	input := `[{"id":"app_123","name":"api-prod","region":{"name":"us"},"team":{"name":"platform"},"space":{"name":"prod"},"stack":{"name":"heroku-22"},"web_url":"https://api-prod.herokuapp.com"}]`
	got := cloudfilter.SummarizeCloudList(input, 6)
	for _, want := range []string{
		"resources: 1",
		"api-prod",
		"region=us",
		"team=platform",
		"space=prod",
		"stack=heroku-22",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Heroku cloud list summary:\n%s", want, got)
		}
	}
}

func TestCloudListRecoveryInfo(t *testing.T) {
	input := `[{"name":"api-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-a","status":"RUNNING","project":"demo-project"},{"name":"jobs-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-b","status":"TERMINATED","project":"demo-project"},{"name":"cron-prod","zone":"https://www.googleapis.com/compute/v1/projects/demo/zones/us-central1-c","status":"RUNNING","project":"demo-project"}]`

	if kind, summary, requireRawCapture := cloudfilter.CloudListRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 1 additional resources" || !requireRawCapture {
		t.Fatalf("unexpected cloud list recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
