package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	tabularprofiles "github.com/devr-tools/szr/internal/profiles/tabular"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCSVTabularProfileRender(t *testing.T) {
	t.Parallel()

	profile := testutil.FindProfile(t, tabularprofiles.Profiles(6), "csv-tabular")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"NAME        READY   STATUS    RESTARTS   AGE   IP           NODE",
			"api-0       1/1     Running   0          3d    10.0.0.12    node-a",
			"worker-0    0/1     Pending   0          5m    <none>       node-b",
		}, "\n"),
	})
	for _, want := range []string{
		"rows: 2 columns: NAME, READY, STATUS, RESTARTS, AGE, IP, NODE",
		"name=api-0 ready=1/1 status=Running age=3d ip=10.0.0.12",
		"name=worker-0 ready=0/1 status=Pending age=5m ip=<none>",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in csv-tabular render output:\n%s", want, rendered)
		}
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("NAME   STATUS   AGE   IP\napi    Running  3d    10.0.0.1\n"))
	streamed.ConsumeStderr([]byte("worker Pending  5m    <none>\n"))
	got := streamed.Result()
	for _, want := range []string{
		"rows: 2 columns: NAME, STATUS, AGE, IP",
		"name=api status=Running age=3d ip=10.0.0.1",
		"name=worker status=Pending age=5m ip=<none>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in csv-tabular stream output:\n%s", want, got)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected csv-tabular stream metadata: %#v", profile)
	}
}

func TestCSVTabularProfileStreamRecovery(t *testing.T) {
	t.Parallel()

	profile := testutil.FindProfile(t, tabularprofiles.Profiles(6), "csv-tabular")
	stream := profile.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"NAME        READY   STATUS    RESTARTS   AGE   IP           NODE",
		"api-0       1/1     Running   0          3d    10.0.0.12    node-a",
		"worker-0    0/1     Pending   0          5m    <none>       node-b",
		"cron-0      1/1     Running   0          1d    10.0.0.13    node-c",
	}, "\n")))
	recoveryStream, ok := stream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable tabular reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recoveryStream.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional rows" || !requireRawCapture {
		t.Fatalf("unexpected tabular recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
