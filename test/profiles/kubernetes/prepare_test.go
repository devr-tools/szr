package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestKubectlProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	kubectlGet := testutil.FindProfile(t, list, "kubectl-get")
	if !kubectlGet.Match(engine.Invocation{Display: []string{"kubectl", "get", "pods"}}) {
		t.Fatal("expected kubectl get to match")
	}
	if !kubectlGet.Match(engine.Invocation{Display: []string{"kubectl", "-n", "default", "get", "pods"}}) {
		t.Fatal("expected namespaced kubectl get to match")
	}
	if kubectlGet.Match(engine.Invocation{Display: []string{"kubectl", "get", "pods", "-o", "wide"}}) {
		t.Fatal("expected explicit wide kubectl get to bypass kubectl-get")
	}
	if got := kubectlGet.Prepare(engine.Invocation{Command: []string{"kubectl", "get", "pods"}}); !reflect.DeepEqual(got, []string{"kubectl", "get", "pods", "-o", "json"}) {
		t.Fatalf("unexpected kubectl get prepare: %#v", got)
	}
	if got := kubectlGet.Prepare(engine.Invocation{Command: []string{"kubectl", "get", "pods", "-o", "yaml"}}); !reflect.DeepEqual(got, []string{"kubectl", "get", "pods", "-o", "yaml"}) {
		t.Fatalf("expected explicit kubectl output to be preserved: %#v", got)
	}

	kubectlLogs := testutil.FindProfile(t, list, "kubectl-logs")
	if !kubectlLogs.Match(engine.Invocation{Display: []string{"kubectl", "--namespace", "prod", "logs", "api"}}) {
		t.Fatal("expected namespaced kubectl logs to match")
	}
	if got := kubectlLogs.Prepare(engine.Invocation{Command: []string{"kubectl", "logs", "api"}}); !reflect.DeepEqual(got, []string{"kubectl", "logs", "--prefix", "--tail=200", "api"}) {
		t.Fatalf("unexpected kubectl logs prepare: %#v", got)
	}
	if got := kubectlLogs.Prepare(engine.Invocation{Command: []string{"kubectl", "logs", "-c", "app", "api"}}); !reflect.DeepEqual(got, []string{"kubectl", "logs", "-c", "app", "--prefix", "--tail=200", "api"}) {
		t.Fatalf("unexpected kubectl logs with container prepare: %#v", got)
	}
	if got := kubectlLogs.Prepare(engine.Invocation{Command: []string{"kubectl", "logs", "--tail=50", "api"}}); !reflect.DeepEqual(got, []string{"kubectl", "logs", "--tail=50", "--prefix", "api"}) {
		t.Fatalf("expected explicit kubectl tail to be preserved: %#v", got)
	}

	kubectlTop := testutil.FindProfile(t, list, "kubectl-top")
	if !kubectlTop.Match(engine.Invocation{Display: []string{"kubectl", "top", "pods"}}) {
		t.Fatal("expected kubectl top to match")
	}
	if got := kubectlTop.Prepare(engine.Invocation{Command: []string{"kubectl", "top", "pods"}}); !reflect.DeepEqual(got, []string{"kubectl", "top", "pods"}) {
		t.Fatalf("expected kubectl top prepare passthrough, got %#v", got)
	}

	kubectlEvents := testutil.FindProfile(t, list, "kubectl-events")
	if !kubectlEvents.Match(engine.Invocation{Display: []string{"kubectl", "events", "-n", "prod"}}) {
		t.Fatal("expected kubectl events to match")
	}
	if got := kubectlEvents.Prepare(engine.Invocation{Command: []string{"kubectl", "events", "-n", "prod"}}); !reflect.DeepEqual(got, []string{"kubectl", "events", "-n", "prod"}) {
		t.Fatalf("expected kubectl events prepare passthrough, got %#v", got)
	}
}
