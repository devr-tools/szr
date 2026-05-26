package tabular_test

import (
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
