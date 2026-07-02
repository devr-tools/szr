package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func firstMatchingContainerProfile(list []engine.Profile, display []string) string {
	for _, profile := range list {
		if profile.Match != nil && profile.Match(engine.Invocation{Display: display}) {
			return profile.Name
		}
	}
	return ""
}

func TestDockerTransferProfile(t *testing.T) {
	list := profiles.Builtins(6)

	dockerTransfer := testutil.FindProfile(t, list, "docker-transfer")
	for _, display := range [][]string{
		{"docker", "pull", "nginx:latest"},
		{"docker", "push", "registry.example.com/app:latest"},
		{"docker", "compose", "pull"},
	} {
		if !dockerTransfer.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %v to match docker-transfer", display)
		}
	}
	if dockerTransfer.Match(engine.Invocation{Display: []string{"docker", "ps"}}) {
		t.Fatal("expected docker ps to bypass docker-transfer")
	}

	rendered := dockerTransfer.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"latest: Pulling from library/nginx",
			"a2abf6c4d29d: Pulling fs layer",
			"a2abf6c4d29d: Downloading [=====>    ]  10.2MB/30.5MB",
			"a2abf6c4d29d: Pull complete",
			"Digest: sha256:abc123",
			"Status: Downloaded newer image for nginx:latest",
		}, "\n"),
	})
	for _, want := range []string{"pulled 1 layers", "Digest: sha256:abc123", "Status: Downloaded newer image for nginx:latest"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in docker-transfer render output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "Downloading") {
		t.Fatalf("expected layer progress to be collapsed:\n%s", rendered)
	}
	if dockerTransfer.StreamPreference != engine.StreamStdoutFirst || dockerTransfer.StreamRender == nil {
		t.Fatalf("unexpected docker-transfer stream metadata: %#v", dockerTransfer)
	}
}

func TestDockerActivityProfile(t *testing.T) {
	list := profiles.Builtins(6)

	dockerActivity := testutil.FindProfile(t, list, "docker-activity")
	for _, display := range [][]string{
		{"docker", "compose", "up", "-d"},
		{"docker", "compose", "build"},
		{"docker", "compose", "down"},
		{"docker", "run", "--rm", "alpine", "true"},
	} {
		if !dockerActivity.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %v to match docker-activity", display)
		}
	}

	// Ordering: existing profiles keep precedence for their commands.
	for _, tc := range []struct {
		display []string
		want    string
	}{
		{[]string{"docker", "compose", "up", "-d"}, "docker-activity"},
		{[]string{"docker", "compose", "build"}, "docker-activity"},
		{[]string{"docker", "pull", "nginx:latest"}, "docker-transfer"},
		{[]string{"docker", "compose", "ps"}, "docker-ps"},
		{[]string{"docker", "compose", "logs", "api"}, "docker-logs"},
		{[]string{"docker", "build", "."}, "build-system"},
	} {
		if got := firstMatchingContainerProfile(list, tc.display); got != tc.want {
			t.Fatalf("expected %v to first-match %q, got %q", tc.display, tc.want, got)
		}
	}

	rendered := dockerActivity.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			" Container myapp-db-1  Starting",
			" Container myapp-db-1  Started",
			" Container myapp-db-1  Healthy",
			" Container myapp-api-1  Started",
		}, "\n"),
		Stderr: "failed to solve: process \"/bin/sh -c make build\" did not complete successfully: exit code: 2",
	})
	for _, want := range []string{
		"services: started=2 healthy=1",
		"Container myapp-db-1 Healthy",
		"Container myapp-api-1 Started",
		"failed to solve",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in docker-activity render output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "Starting") {
		t.Fatalf("expected intermediate container states to be dropped:\n%s", rendered)
	}
	if dockerActivity.StreamPreference != engine.StreamStdoutFirst || dockerActivity.StreamRender == nil {
		t.Fatalf("unexpected docker-activity stream metadata: %#v", dockerActivity)
	}
}
