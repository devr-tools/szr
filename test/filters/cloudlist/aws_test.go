package cloudlist_test

import (
	"fmt"
	"strings"
	"testing"

	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlist"
)

func TestSummarizeAWSLambdaFunctions(t *testing.T) {
	input := `{"Functions":[
		{"FunctionName":"invoice-webhook","Runtime":"python3.12","CodeSize":2312960,"MemorySize":512,"Timeout":30,"Handler":"app.handler"},
		{"FunctionName":"receipt-renderer","Runtime":"nodejs20.x","CodeSize":73400320,"MemorySize":2048,"Timeout":120,"Handler":"render.handler"}
	]}`
	got := cloudfilter.SummarizeAWSLambdaFunctions(input, 6)
	for _, want := range []string{
		"functions: 2 (python3.12=1 nodejs20.x=1)",
		"invoice-webhook runtime=python3.12 size=2.2MB mem=512MB timeout=30s",
		"receipt-renderer runtime=nodejs20.x size=70.0MB mem=2048MB timeout=120s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in lambda summary:\n%s", want, got)
		}
	}

	if got := cloudfilter.SummarizeAWSLambdaFunctions(`[{"name":"api","status":"RUNNING"}]`, 6); !strings.Contains(got, "resources: 1") {
		t.Fatalf("expected generic cloud-list fallback for unknown shapes, got:\n%s", got)
	}
}

func TestSummarizeAWSECS(t *testing.T) {
	describe := `{"services":[
		{"serviceName":"billing-api","status":"ACTIVE","runningCount":2,"desiredCount":3,"pendingCount":1,"deployments":[{"rolloutState":"IN_PROGRESS"}]},
		{"serviceName":"ledger-worker","status":"ACTIVE","runningCount":4,"desiredCount":4,"pendingCount":0,"deployments":[{"rolloutState":"COMPLETED"}]}
	],"failures":[{"arn":"arn:aws:ecs:us-east-1:000000000000:service/core/ghost-service","reason":"MISSING"}]}`
	got := cloudfilter.SummarizeAWSECS(describe, 8)
	for _, want := range []string{
		"services: 3",
		"failure: ghost-service MISSING",
		"billing-api status=ACTIVE running=2/3 pending=1 rollout=IN_PROGRESS",
		"ledger-worker status=ACTIVE running=4/4",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in ecs describe summary:\n%s", want, got)
		}
	}

	arns := `{"serviceArns":["arn:aws:ecs:us-east-1:000000000000:service/core/billing-api","arn:aws:ecs:us-east-1:000000000000:service/core/ledger-worker"]}`
	got = cloudfilter.SummarizeAWSECS(arns, 6)
	for _, want := range []string{"services: 2", "billing-api", "ledger-worker"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in ecs arn summary:\n%s", want, got)
		}
	}
}

func TestSummarizeAWSS3Ls(t *testing.T) {
	lines := []string{"                           PRE logs/", "                           PRE tmp/"}
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("2026-07-%02d 09:14:22      %d backups/db-2026-07-%02d.tar.gz", i+1, 1024*(i+1), i+1))
	}
	lines = append(lines, "2026-06-01 03:00:00 5368709120 backups/full-archive.tar.gz")
	got := cloudfilter.SummarizeAWSS3Ls(strings.Join(lines, "\n"), 6)
	for _, want := range []string{
		"objects: 31 (total 5.0GB) prefixes: 2",
		"5.0GB backups/full-archive.tar.gz (2026-06-01)",
		"... +26 smaller objects",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in s3 ls summary:\n%s", want, got)
		}
	}

	buckets := "2026-01-11 08:00:12 invoice-archive\n2026-02-20 16:45:31 billing-reports\n"
	got = cloudfilter.SummarizeAWSS3Ls(buckets, 6)
	for _, want := range []string{"buckets: 2", "invoice-archive", "billing-reports"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in s3 bucket summary:\n%s", want, got)
		}
	}
}

func TestSummarizeAWSLogEvents(t *testing.T) {
	events := []string{}
	for i := 0; i < 40; i++ {
		events = append(events, fmt.Sprintf(`{"logStreamName":"app/instance-a","timestamp":%d,"message":"processed invoice %d in %dms"}`, 1720000000000+i, 4000+i, 12+i))
	}
	events = append(events,
		`{"logStreamName":"app/instance-b","timestamp":1720000009000,"message":"ERROR failed to persist invoice 4021: connection reset"}`,
		`{"logStreamName":"app/instance-b","timestamp":1720000009500,"message":"ERROR failed to persist invoice 4022: connection reset"}`,
	)
	input := fmt.Sprintf(`{"events":[%s]}`, strings.Join(events, ","))

	got := cloudfilter.SummarizeAWSLogEvents(input, 6)
	for _, want := range []string{
		"log events: 42 (errors=2) streams=2",
		"x2 ERROR failed to persist invoice 4021: connection reset",
		"x40 processed invoice 4000 in 12ms",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in log events summary:\n%s", want, got)
		}
	}

	if got := cloudfilter.SummarizeAWSLogEvents("plain text\n", 4); !strings.Contains(got, "plain text") {
		t.Fatalf("expected compact fallback for non-JSON input, got:\n%s", got)
	}
}

func TestSummarizeAWSStackEvents(t *testing.T) {
	events := []string{
		`{"ResourceStatus":"UPDATE_IN_PROGRESS","LogicalResourceId":"billing-stack","ResourceType":"AWS::CloudFormation::Stack"}`,
		`{"ResourceStatus":"CREATE_FAILED","LogicalResourceId":"invoices-table","ResourceType":"AWS::DynamoDB::Table","ResourceStatusReason":"Table already exists: invoices-table"}`,
	}
	for i := 0; i < 6; i++ {
		events = append(events, fmt.Sprintf(`{"ResourceStatus":"CREATE_FAILED","LogicalResourceId":"resource-%d","ResourceType":"AWS::SNS::Topic","ResourceStatusReason":"Resource creation cancelled"}`, i))
	}
	for i := 0; i < 12; i++ {
		events = append(events, fmt.Sprintf(`{"ResourceStatus":"CREATE_COMPLETE","LogicalResourceId":"ok-resource-%d","ResourceType":"AWS::SSM::Parameter"}`, i))
	}
	input := fmt.Sprintf(`{"StackEvents":[%s]}`, strings.Join(events, ","))

	got := cloudfilter.SummarizeAWSStackEvents(input, 6)
	for _, want := range []string{
		"stack events: 20 (failed=7)",
		"CREATE_FAILED invoices-table (AWS::DynamoDB::Table): Table already exists: invoices-table",
		"CREATE_FAILED resource-0 (AWS::SNS::Topic): Resource creation cancelled (x6)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in stack events summary:\n%s", want, got)
		}
	}

	kind, summary, requireRaw := cloudfilter.AWSStackEventsRecoveryInfo(input, 6)
	if kind != "full-output" || !strings.Contains(summary, "additional stack events") || !requireRaw {
		t.Fatalf("unexpected stack events recovery info: kind=%q summary=%q requireRaw=%v", kind, summary, requireRaw)
	}
}
