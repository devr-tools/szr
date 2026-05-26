package sqlquery_test

import (
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
