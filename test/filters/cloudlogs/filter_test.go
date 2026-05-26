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

func TestSummarizeCloudLogsOCIResults(t *testing.T) {
	t.Parallel()

	input := `{"data":{"results":[{"timestamp":"2026-05-25T10:00:00Z","severity":"ERROR","message":"rate limit exceeded","source":"api-gateway"},{"timestamp":"2026-05-25T10:00:10Z","severity":"ERROR","message":"rate limit exceeded","source":"api-gateway"}]}}`
	got := cloudfilter.SummarizeCloudLogs(input, 5)
	for _, want := range []string{
		"events: 2 sources: 1",
		"services: api-gateway",
		"api-gateway: ERROR rate limit exceeded (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in OCI cloud log summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudLogsSupabaseStructured(t *testing.T) {
	t.Parallel()

	input := `[{"timestamp":"2026-05-25T10:00:00Z","level":"ERROR","event_message":"connection reset","service":"postgres"},{"timestamp":"2026-05-25T10:00:05Z","level":"ERROR","event_message":"connection reset","service":"postgres"}]`
	got := cloudfilter.SummarizeCloudLogs(input, 5)
	for _, want := range []string{
		"events: 2 sources: 1",
		"services: postgres",
		"postgres: ERROR connection reset (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Supabase cloud log summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudLogsSupabaseFunctionErrors(t *testing.T) {
	t.Parallel()

	input := `[{"timestamp":"2026-05-25T10:00:00Z","level":"ERROR","function_slug":"stripe-webhook","status_code":"500","method":"POST","path":"/webhook","msg":"request failed"},{"timestamp":"2026-05-25T10:00:01Z","level":"ERROR","function_slug":"stripe-webhook","status_code":"500","method":"POST","path":"/webhook","msg":"request failed"}]`
	got := cloudfilter.SummarizeCloudLogs(input, 5)
	for _, want := range []string{
		"events: 2 sources: 1",
		"services: stripe-webhook",
		"stripe-webhook: ERROR 500 POST /webhook request failed (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Supabase function log summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudLogsVercelText(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"2026-05-25T10:00:00Z iad1 api ERROR Function Invocation failed: timeout",
		"2026-05-25T10:00:01Z iad1 api ERROR Function Invocation failed: timeout",
		"2026-05-25T10:00:02Z iad1 api INFO request completed",
	}, "\n")
	got := cloudfilter.SummarizeCloudLogs(input, 5)
	for _, want := range []string{
		"events: 3 sources: 1",
		"services: api",
		"api: ERROR Function Invocation failed: timeout (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Vercel log summary:\n%s", want, got)
		}
	}
}

func TestSummarizeCloudLogsHerokuText(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"2026-05-25T10:00:00Z heroku[router]: at=error code=H12 desc=\"Request timeout\" method=GET path=\"/checkout\" dyno=web.1 status=503 service=30000ms",
		"2026-05-25T10:00:03Z app[web.1]: Error R14 (Memory quota exceeded)",
		"2026-05-25T10:00:04Z app[web.1]: Error R14 (Memory quota exceeded)",
	}, "\n")
	got := cloudfilter.SummarizeCloudLogs(input, 6)
	for _, want := range []string{
		"events: 3 sources: 2",
		"services: app/web.1, heroku/router",
		"heroku/router: code=H12 status=503 dyno=web.1 path=/checkout",
		"app/web.1: ERROR R14 Memory quota exceeded (x2)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in Heroku cloud log summary:\n%s", want, got)
		}
	}
}
