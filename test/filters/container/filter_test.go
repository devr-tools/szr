package container_test

import (
	"strings"
	"testing"

	containerfilter "github.com/devr-tools/szr/internal/filters/container"
)

func TestSummarizeDockerPS(t *testing.T) {
	tabular := "api\tUp 3 minutes\tapp:latest\nworker\tExited (1) 10 seconds ago\tworker:latest\n"
	got := containerfilter.SummarizeDockerPS(tabular, 4)
	for _, want := range []string{
		"containers: running=1 exited=1 other=0",
		"api: Up 3 minutes [app:latest]",
		"worker: Exited (1) 10 seconds ago [worker:latest]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in docker ps summary:\n%s", want, got)
		}
	}

	json := `[{"Service":"api","State":"running","Health":"healthy","Image":"app:latest"},{"Service":"worker","State":"exited","Image":"worker:latest"}]`
	got = containerfilter.SummarizeDockerPS(json, 4)
	for _, want := range []string{
		"containers: running=1 exited=1 other=0",
		"api: running (healthy) [app:latest]",
		"worker: exited [worker:latest]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in compose ps summary:\n%s", want, got)
		}
	}
}

func TestSummarizeDockerLogs(t *testing.T) {
	input := strings.Join([]string{
		"api-1  | INFO listening on :8080",
		"api-1  | ERROR failed to connect to db",
		"api-1  | ERROR failed to connect to db",
		"worker-1  | panic: bad queue state",
		"worker-1  | panic: bad queue state",
	}, "\n")

	got := containerfilter.SummarizeDockerLogs(input, 5)
	for _, want := range []string{
		"sources: 2",
		"api-1: ERROR failed to connect to db (x2)",
		"worker-1: panic: bad queue state (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in docker logs summary:\n%s", want, got)
		}
	}
}

func TestDockerRecoveryInfo(t *testing.T) {
	psInput := strings.Join([]string{
		"api\tUp 3 minutes\tapp:latest",
		"worker\tExited (1) 10 seconds ago\tworker:latest",
		"cron\tUp 1 minute\tcron:latest",
	}, "\n")
	if kind, summary, requireRawCapture := containerfilter.DockerPSRecoveryInfo(psInput, 2); kind != "full-output" || summary != "omitted 2 additional containers" || !requireRawCapture {
		t.Fatalf("unexpected docker ps recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	logsInput := strings.Join([]string{
		"api-1  | ERROR failed to connect to db",
		"api-1  | WARN backing off",
		"worker-1  | panic: bad queue state",
		"worker-1  | fatal: exiting",
	}, "\n")
	if kind, summary, requireRawCapture := containerfilter.DockerLogsRecoveryInfo(logsInput, 3); kind != "full-output" || summary != "omitted 2 additional log lines" || !requireRawCapture {
		t.Fatalf("unexpected docker logs recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
