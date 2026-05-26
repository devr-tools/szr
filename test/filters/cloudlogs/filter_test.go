package cloudlogs_test

import (
	"strings"
	"testing"

	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlogs"
)

func TestSummarizeCloudLogsText(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"2026-05-25T10:00:00Z api ERROR timeout talking to redis",
		"2026-05-25T10:00:05Z api ERROR timeout talking to redis",
		"2026-05-25T10:01:00Z worker WARN retry scheduled",
	}, "\n")
	got := cloudfilter.SummarizeCloudLogs(input, 6)
	for _, want := range []string{
		"events: 3 sources: 2",
		"time: 2026-05-25T10:00:00Z .. 2026-05-25T10:01:00Z",
		"services: api, worker",
		"api: ERROR timeout talking to redis (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cloud log summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudLogsStructured(t *testing.T) {
	t.Parallel()

	input := `[{"timestamp":"2026-05-25T10:00:00Z","severity":"ERROR","textPayload":"request failed","resource":{"type":"k8s_container","labels":{"container_name":"api"}}},{"timestamp":"2026-05-25T10:00:10Z","severity":"ERROR","textPayload":"request failed","resource":{"type":"k8s_container","labels":{"container_name":"api"}}}]`
	got := cloudfilter.SummarizeCloudLogs(input, 5)
	for _, want := range []string{
		"events: 2 sources: 1",
		"services: k8s_container",
		"k8s_container: ERROR request failed (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in structured cloud log summary:\n%s", want, got)
		}
	}
}
