package container_test

import (
	"strings"
	"testing"

	containerfilter "github.com/devr-tools/szr/internal/filters/container"
)

func TestSummarizeDockerPull(t *testing.T) {
	input := strings.Join([]string{
		"latest: Pulling from library/nginx",
		"a2abf6c4d29d: Pulling fs layer",
		"b1c3d5e7f9a0: Pulling fs layer",
		"c4d6e8f0a2b4: Already exists",
		"a2abf6c4d29d: Downloading [=====>    ]  10.2MB/30.5MB",
		"a2abf6c4d29d: Extracting [====>     ]  5.1MB/30.5MB",
		"a2abf6c4d29d: Pull complete",
		"b1c3d5e7f9a0: Pull complete",
		"Digest: sha256:abc123",
		"Status: Downloaded newer image for nginx:latest",
		"docker.io/library/nginx:latest",
	}, "\n")

	got := containerfilter.SummarizeDockerTransfer(input, 8)
	for _, want := range []string{
		"pulled 2 layers (1 already existed)",
		"latest: Pulling from library/nginx",
		"Digest: sha256:abc123",
		"Status: Downloaded newer image for nginx:latest",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in docker pull summary:\n%s", want, got)
		}
	}
	for _, reject := range []string{"Downloading", "Extracting", "Pull complete"} {
		if strings.Contains(got, reject) {
			t.Fatalf("expected per-layer progress %q to be collapsed:\n%s", reject, got)
		}
	}
}

func TestSummarizeDockerPush(t *testing.T) {
	input := strings.Join([]string{
		"The push refers to repository [registry.example.com/app]",
		"5f70bf18a086: Preparing",
		"8d3ac3489996: Preparing",
		"5f70bf18a086: Layer already exists",
		"8d3ac3489996: Pushed",
		"latest: digest: sha256:def456 size: 1234",
	}, "\n")

	got := containerfilter.SummarizeDockerTransfer(input, 8)
	for _, want := range []string{
		"pushed 1 layers (1 already existed)",
		"The push refers to repository [registry.example.com/app]",
		"latest: digest: sha256:def456 size: 1234",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in docker push summary:\n%s", want, got)
		}
	}
}

func TestSummarizeDockerPullError(t *testing.T) {
	input := strings.Join([]string{
		"Using default tag: latest",
		"Error response from daemon: pull access denied for private/app, repository does not exist or may require 'docker login'",
	}, "\n")

	got := containerfilter.SummarizeDockerTransfer(input, 6)
	if !strings.Contains(got, "Error response from daemon: pull access denied") {
		t.Fatalf("expected registry error to be kept:\n%s", got)
	}
}

func TestSummarizeComposeActivity(t *testing.T) {
	input := strings.Join([]string{
		" Network myapp_default  Creating",
		" Network myapp_default  Created",
		" Container myapp-db-1  Creating",
		" Container myapp-db-1  Created",
		" Container myapp-db-1  Starting",
		" Container myapp-db-1  Started",
		" Container myapp-db-1  Healthy",
		" Container myapp-api-1  Started",
		"db-1   | ready to accept connections",
		"api-1  | INFO listening on :8080",
		"api-1  | ERROR failed to connect to db",
	}, "\n")

	got := containerfilter.SummarizeComposeActivity(input, 8)
	for _, want := range []string{
		"services: started=2 healthy=1",
		"Network myapp_default Created",
		"Container myapp-db-1 Healthy",
		"Container myapp-api-1 Started",
		"api-1  | ERROR failed to connect to db",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in compose activity summary:\n%s", want, got)
		}
	}
	for _, reject := range []string{"Creating", "Starting", "INFO listening", "ready to accept"} {
		if strings.Contains(got, reject) {
			t.Fatalf("expected progress/attach noise %q to be dropped:\n%s", reject, got)
		}
	}
}

func TestSummarizeComposeBuildFailure(t *testing.T) {
	input := strings.Join([]string{
		"#1 [internal] load build definition from Dockerfile",
		"#1 DONE 0.0s",
		"#8 [api 4/6] RUN make build",
		"#8 1.234 make: *** [Makefile:12: build] Error 2",
		`#8 ERROR: process "/bin/sh -c make build" did not complete successfully: exit code: 2`,
		`failed to solve: process "/bin/sh -c make build" did not complete successfully: exit code: 2`,
	}, "\n")

	got := containerfilter.SummarizeComposeActivity(input, 8)
	for _, want := range []string{
		"#8 1.234 make: *** [Makefile:12: build] Error 2",
		`#8 ERROR: process "/bin/sh -c make build" did not complete successfully: exit code: 2`,
		"failed to solve",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in compose build failure summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "load build definition") {
		t.Fatalf("expected BuildKit progress noise to be dropped:\n%s", got)
	}
}

func TestDockerProgressRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		" Container myapp-db-1  Started",
		" Container myapp-api-1  Started",
		" Container myapp-cron-1  Started",
		" Container myapp-web-1  Started",
	}, "\n")
	if kind, summary, requireRawCapture := containerfilter.ComposeActivityRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected compose activity recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
