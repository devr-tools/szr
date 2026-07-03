package sqlquery_test

import (
	"fmt"
	"strings"
	"testing"

	sqlfilter "github.com/devr-tools/szr/internal/filters/sqlquery"
)

func TestSummarizeSQLQueryTableOutput(t *testing.T) {
	input := strings.Join([]string{
		"+----+-------+",
		"| id | name  |",
		"+----+-------+",
		"| 1  | alpha |",
		"| 2  | beta  |",
		"+----+-------+",
		"2 rows in set (0.01 sec)",
	}, "\n")

	got := sqlfilter.SummarizeSQLQuery(input, 4)
	for _, want := range []string{
		"| id | name  |",
		"| 1  | alpha |",
		"| 2  | beta  |",
		"2 rows in set (0.01 sec)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in SQL summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "+----+-------+") {
		t.Fatalf("expected table borders to be removed:\n%s", got)
	}
}

func TestSummarizeSQLQueryErrorOutput(t *testing.T) {
	input := strings.Join([]string{
		"ERROR: relation \"missing\" does not exist",
		"LINE 1: select * from missing;",
	}, "\n")

	got := sqlfilter.SummarizeSQLQuery(input, 4)
	for _, want := range []string{
		"ERROR: relation \"missing\" does not exist",
		"LINE 1: select * from missing;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in SQL error summary:\n%s", want, got)
		}
	}
}

func TestSummarizeSQLQueryJSONOutput(t *testing.T) {
	input := `[{"id":1,"name":"alpha"},{"id":2,"name":"beta"}]`

	got := sqlfilter.SummarizeSQLQuery(input, 3)
	for _, want := range []string{
		"2 row(s)",
		`{"id":1,"name":"alpha"}`,
		`{"id":2,"name":"beta"}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in SQL JSON summary:\n%s", want, got)
		}
	}
}

// TestSummarizeSQLQueryKeepsAnomalousRows pins the anomaly rule for query
// results: a row carrying a rare value in a low-cardinality column (the
// status-shaped needle a SELECT is usually hunting for) must survive
// truncation, and the exact row count must stay visible.
func TestSummarizeSQLQueryKeepsAnomalousRows(t *testing.T) {
	lines := []string{
		"  id  |          email           |  plan   |  status  ",
		"------+--------------------------+---------+----------",
	}
	for i := 0; i < 40; i++ {
		status := "active"
		if i == 31 {
			status = "suspended"
		}
		email := fmt.Sprintf("member%03d@example.com", i)
		if i == 31 {
			email = "stale.billing@example.com"
		}
		plan := []string{"basic", "team", "scale"}[i%3]
		lines = append(lines, fmt.Sprintf(" %4d | %-24s | %-7s | %s", 5000+i, email, plan, status))
	}
	lines = append(lines, "(40 rows)")

	got := sqlfilter.SummarizeSQLQuery(strings.Join(lines, "\n"), 10)
	for _, want := range []string{
		"stale.billing@example.com",
		"suspended",
		"(40 rows)",
		"more rows",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in anomaly-aware SQL summary:\n%s", want, got)
		}
	}
}

// TestSummarizeSQLQueryJSONKeepsAnomalousRecords pins the same rule for JSON
// result sets (sqlite3 -json / duckdb -json).
func TestSummarizeSQLQueryJSONKeepsAnomalousRecords(t *testing.T) {
	records := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		state := "shipped"
		if i == 17 {
			state = "lost"
		}
		records = append(records, fmt.Sprintf(`{"order_id":%d,"state":%q}`, 9000+i, state))
	}
	input := "[" + strings.Join(records, ",") + "]"

	got := sqlfilter.SummarizeSQLQuery(input, 6)
	for _, want := range []string{
		"20 row(s)",
		`"order_id":9017`,
		`"state":"lost"`,
		"more rows",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in anomaly-aware SQL JSON summary:\n%s", want, got)
		}
	}
}

func TestSQLQueryRecoveryInfo(t *testing.T) {
	tableInput := strings.Join([]string{
		"| id | name  |",
		"| 1  | alpha |",
		"| 2  | beta  |",
		"| 3  | gamma |",
		"3 rows in set (0.01 sec)",
	}, "\n")
	if kind, summary, requireRawCapture := sqlfilter.SQLQueryRecoveryInfo(tableInput, 3); kind != "full-output" || summary != "omitted 2 additional rows or lines" || !requireRawCapture {
		t.Fatalf("unexpected SQL table recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	jsonInput := `[{"id":1,"name":"alpha"},{"id":2,"name":"beta"},{"id":3,"name":"gamma"}]`
	if kind, summary, requireRawCapture := sqlfilter.SQLQueryRecoveryInfo(jsonInput, 2); kind != "full-output" || summary != "omitted 2 additional rows or lines" || !requireRawCapture {
		t.Fatalf("unexpected SQL JSON recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
