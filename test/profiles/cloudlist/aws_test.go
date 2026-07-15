package profiles_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	cloudprofiles "github.com/devr-tools/szr/internal/profiles/cloudlist"
	"github.com/devr-tools/szr/test/testutil"
)

func TestAWSProfilesMatch(t *testing.T) {
	list := cloudprofiles.Profiles(6)

	for name, display := range map[string][]string{
		"aws-log-events":       {"aws", "logs", "filter-log-events", "--log-group-name", "/aws/lambda/api"},
		"aws-lambda-functions": {"aws", "lambda", "list-functions"},
		"aws-stack-events":     {"aws", "cloudformation", "describe-stack-events", "--stack-name", "billing"},
		"aws-s3-ls":            {"aws", "s3", "ls", "s3://invoice-archive/2026/"},
		"aws-ecs-state":        {"aws", "--profile", "dev", "ecs", "describe-services", "--cluster", "core"},
	} {
		profile := testutil.FindProfile(t, list, name)
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match %s", display, name)
		}
	}

	logEvents := testutil.FindProfile(t, list, "aws-log-events")
	for _, display := range [][]string{
		{"aws", "logs", "tail", "/aws/lambda/api"},
		{"aws", "lambda", "list-functions"},
		{"aws", "logs"},
	} {
		if logEvents.Match(engine.Invocation{Display: display}) {
			t.Fatalf("did not expect %#v to match aws-log-events", display)
		}
	}

	if got := testutil.FindProfile(t, list, "aws-log-events").Prepare(engine.Invocation{Command: []string{"aws", "logs", "get-log-events", "--log-stream-name", "app"}}); !reflect.DeepEqual(got, []string{"aws", "logs", "get-log-events", "--log-stream-name", "app", "--output", "json"}) {
		t.Fatalf("unexpected aws-log-events prepare: %#v", got)
	}
	if got := testutil.FindProfile(t, list, "aws-lambda-functions").Prepare(engine.Invocation{Command: []string{"aws", "lambda", "list-functions", "--output", "table"}}); !reflect.DeepEqual(got, []string{"aws", "lambda", "list-functions", "--output", "table"}) {
		t.Fatalf("expected explicit output format to be preserved, got %#v", got)
	}
}

func TestAWSProfilesRouteAheadOfGenericCloudProfiles(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	e := engine.New(cfg, paths, nil, profiles.Builtins(cfg.MaxPreviewLines))

	for display, want := range map[string]string{
		"aws logs filter-log-events --log-group-name /aws/lambda/api": "aws-log-events",
		"aws logs get-log-events --log-stream-name app":               "aws-log-events",
		"aws lambda list-functions":                                   "aws-lambda-functions",
		"aws cloudformation describe-stack-events":                    "aws-stack-events",
		"aws s3 ls":             "aws-s3-ls",
		"aws ecs list-services": "aws-ecs-state",
		"aws ecs describe-services --cluster core":    "aws-ecs-state",
		"aws ec2 describe-instances":                  "cloud-list",
		"aws lambda get-function --function-name api": "cloud-list",
		"aws logs tail /aws/lambda/api":               "cloud-logs",
	} {
		inv := engine.Invocation{Command: strings.Fields(display), Display: strings.Fields(display)}
		if got := e.Explain(inv).Name; got != want {
			t.Fatalf("expected %q to route to %q, got %q", display, want, got)
		}
	}
}

func TestAWSProfilesRenderAndStream(t *testing.T) {
	list := cloudprofiles.Profiles(6)

	lambda := testutil.FindProfile(t, list, "aws-lambda-functions")
	rendered := lambda.Render(engine.Invocation{}, engine.Execution{
		Stdout: `{"Functions":[{"FunctionName":"invoice-webhook","Runtime":"python3.12","CodeSize":2312960,"MemorySize":512,"Timeout":30}]}`,
	})
	for _, want := range []string{"functions: 1", "invoice-webhook runtime=python3.12 size=2.2MB mem=512MB timeout=30s"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in lambda render:\n%s", want, rendered)
		}
	}

	ecs := testutil.FindProfile(t, list, "aws-ecs-state")
	if ecs.StreamPreference != engine.StreamStdoutOnly || ecs.StreamRender == nil {
		t.Fatalf("unexpected aws-ecs-state stream metadata: %#v", ecs)
	}
	stream := ecs.StreamRender(engine.Invocation{}, ecs.Budget)
	stream.ConsumeStdout([]byte(`{"services":[{"serviceName":"billing-api","status":"ACTIVE","runningCount":2,"desiredCount":3,"pendingCount":1}],"failures":[]}`))
	got := stream.Result()
	for _, want := range []string{"services: 1", "billing-api status=ACTIVE running=2/3 pending=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in ecs streamed output:\n%s", want, got)
		}
	}
}
