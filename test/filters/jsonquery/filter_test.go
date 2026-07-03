package jsonquery_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters/jsonquery"
)

func TestSummarizeQueryOutput(t *testing.T) {
	t.Parallel()

	rendered := jsonquery.SummarizeQueryOutput(`{"user":{"id":7,"name":"alex"},"items":[1,2]}`, "", 6)
	for _, want := range []string{`root: object keys=2`, `user: object keys=2`, `user.id=7`, `items: array len=2 sample=1, 2`} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered json output:\n%s", want, rendered)
		}
	}

	ndjson := jsonquery.SummarizeQueryOutput(strings.Join([]string{
		`{"id":1,"name":"one"}`,
		`{"id":2,"name":"two"}`,
	}, "\n"), "", 4)
	for _, want := range []string{`id=1`, `name="one"`, `id=2`, `name="two"`} {
		if !strings.Contains(ndjson, want) {
			t.Fatalf("expected %q in ndjson output:\n%s", want, ndjson)
		}
	}

	fallback := jsonquery.SummarizeQueryOutput("", "jq: parse error: Invalid numeric literal at line 1, column 4", 4)
	if !strings.Contains(fallback, "jq: parse error") {
		t.Fatalf("expected stderr fallback, got:\n%s", fallback)
	}
}

func TestSummarizeQueryOutputTabularUniformArray(t *testing.T) {
	t.Parallel()

	items := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		state := "open"
		if i == 27 {
			state = "spam_flagged"
		}
		items = append(items, fmt.Sprintf(`{"id":%d,"state":%q,"author":"alice"}`, 9000+i, state))
	}

	rendered := jsonquery.SummarizeQueryOutput("["+strings.Join(items, ",")+"]", "", 8)
	for _, want := range []string{
		"array len=40 uniform objects, cols: id|state|author",
		"9000|open|alice",
		"#27 9027|spam_flagged|alice",
		"more rows",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in tabular array output:\n%s", want, rendered)
		}
	}

	ndjson := jsonquery.SummarizeQueryOutput(strings.Join(items, "\n"), "", 8)
	for _, want := range []string{
		"stream len=40 uniform objects, cols: id|state|author",
		"#27 9027|spam_flagged|alice",
	} {
		if !strings.Contains(ndjson, want) {
			t.Fatalf("expected %q in tabular stream output:\n%s", want, ndjson)
		}
	}

	kind, summary, requireRawCapture := jsonquery.QueryOutputRecoveryInfo("["+strings.Join(items, ",")+"]", "", 8)
	if kind != "full-output" || !strings.Contains(summary, "additional lines") || !requireRawCapture {
		t.Fatalf("unexpected tabular recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestQueryOutputRecoveryInfo(t *testing.T) {
	input := `{"user":{"id":7,"name":"alex","team":{"id":"t_1","name":"platform"}},"items":[1,2,3],"meta":{"page":1,"total":10}}`

	if kind, summary, requireRawCapture := jsonquery.QueryOutputRecoveryInfo(input, "", 4); kind != "full-output" || summary != "omitted 7 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected json query recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
