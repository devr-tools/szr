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
