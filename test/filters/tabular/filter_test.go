package tabular_test

import (
	"fmt"
	"strings"
	"testing"

	tabularfilter "github.com/devr-tools/szr/internal/filters/tabular"
)

func TestSummarizeWideTable(t *testing.T) {
	t.Parallel()

	ps := strings.Join([]string{
		"PID  PPID USER      %CPU %MEM ELAPSED COMMAND",
		"101  1    root      0.1  1.2  01:23:45 /usr/libexec/sysmond --serve",
		"222  101  postgres  2.5  3.7  00:04:10 postgres: writer process",
	}, "\n")
	got := tabularfilter.SummarizeWideTable(ps, 4)
	for _, want := range []string{
		"rows: 2 columns: PID, PPID, USER, %CPU, %MEM, ELAPSED, COMMAND",
		"pid=101 cpu=0.1 mem=1.2 elapsed=01:23:45 command=/usr/libexec/sysmond --serve",
		"pid=222 cpu=2.5 mem=3.7 elapsed=00:04:10 command=postgres: writer process",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in tabular summary:\n%s", want, got)
		}
	}
}

func TestSummarizeWideTableDUFallback(t *testing.T) {
	t.Parallel()

	du := "120\t./cmd\n48\t./internal\n"
	got := tabularfilter.SummarizeWideTable(du, 4)
	for _, want := range []string{
		"rows: 2 columns: SIZE, PATH",
		"path=./cmd size=120",
		"path=./internal size=48",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in du summary:\n%s", want, got)
		}
	}
}

// TestSummarizeWideTableKeepsAnomalousRows pins the anomaly rule for wide
// tables: the row with a minority value in a low-cardinality column (a
// crashing pod among running ones) must survive positional truncation.
func TestSummarizeWideTableKeepsAnomalousRows(t *testing.T) {
	t.Parallel()

	lines := []string{"NAME          READY   STATUS             RESTARTS   AGE"}
	for i := 0; i < 14; i++ {
		status := "Running"
		ready := "1/1"
		restarts := "0"
		if i == 11 {
			status = "CrashLoopBackOff"
			ready = "0/1"
			restarts = "37"
		}
		lines = append(lines, fmt.Sprintf("api-%02d        %s     %-18s %-10s 4d", i, ready, status, restarts))
	}

	got := tabularfilter.SummarizeWideTable(strings.Join(lines, "\n"), 6)
	for _, want := range []string{
		"rows: 14",
		"api-11",
		"status=CrashLoopBackOff",
		"more rows",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in anomaly-aware tabular summary:\n%s", want, got)
		}
	}
}

func TestWideTableRecoveryInfo(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"NAME        READY   STATUS    RESTARTS   AGE   IP           NODE",
		"api-0       1/1     Running   0          3d    10.0.0.12    node-a",
		"worker-0    0/1     Pending   0          5m    <none>       node-b",
		"cron-0      1/1     Running   0          1d    10.0.0.13    node-c",
	}, "\n")
	if kind, summary, requireRawCapture := tabularfilter.WideTableRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 1 additional rows" || !requireRawCapture {
		t.Fatalf("unexpected wide table recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
