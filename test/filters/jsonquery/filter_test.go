package jsonquery_test

import (
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
