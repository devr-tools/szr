package profiles_test

import (
	"reflect"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestDockerProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	dockerPS := testutil.FindProfile(t, list, "docker-ps")
	if !dockerPS.Match(engine.Invocation{Display: []string{"docker", "ps"}}) {
		t.Fatal("expected docker ps to match")
	}
	if !dockerPS.Match(engine.Invocation{Display: []string{"docker", "compose", "ps"}}) {
		t.Fatal("expected docker compose ps to match")
	}
	if got := dockerPS.Prepare(engine.Invocation{Command: []string{"docker", "ps"}}); !reflect.DeepEqual(got, []string{"docker", "ps", "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}"}) {
		t.Fatalf("unexpected docker ps prepare: %#v", got)
	}
	if got := dockerPS.Prepare(engine.Invocation{Command: []string{"docker", "compose", "ps"}}); !reflect.DeepEqual(got, []string{"docker", "compose", "ps", "--format", "json"}) {
		t.Fatalf("unexpected docker compose ps prepare: %#v", got)
	}

	dockerLogs := testutil.FindProfile(t, list, "docker-logs")
	if !dockerLogs.Match(engine.Invocation{Display: []string{"docker", "logs", "api"}}) {
		t.Fatal("expected docker logs to match")
	}
	if !dockerLogs.Match(engine.Invocation{Display: []string{"docker", "compose", "logs", "api"}}) {
		t.Fatal("expected docker compose logs to match")
	}
	if got := dockerLogs.Prepare(engine.Invocation{Command: []string{"docker", "logs", "api"}}); !reflect.DeepEqual(got, []string{"docker", "logs", "--tail", "200", "api"}) {
		t.Fatalf("unexpected docker logs prepare: %#v", got)
	}
	if got := dockerLogs.Prepare(engine.Invocation{Command: []string{"docker", "compose", "logs", "api"}}); !reflect.DeepEqual(got, []string{"docker", "compose", "logs", "--tail", "200", "api"}) {
		t.Fatalf("unexpected docker compose logs prepare: %#v", got)
	}
	if got := dockerLogs.Prepare(engine.Invocation{Command: []string{"docker", "logs", "--tail", "50", "api"}}); !reflect.DeepEqual(got, []string{"docker", "logs", "--tail", "50", "api"}) {
		t.Fatalf("expected explicit docker tail to be preserved: %#v", got)
	}
}
