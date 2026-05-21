package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestGoJSONSummary(t *testing.T) {
	t.Parallel()

	goJSON := filters.SummarizeGoTestJSON(strings.Join([]string{
		`{"Action":"pass","Package":"pkg/pass"}`,
		`{"Action":"fail","Package":"pkg/fail"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"TestOne"}`,
		`{"Action":"output","Package":"pkg/fail","Test":"TestOne","Output":"panic: boom"}`,
	}, "\n"))
	for _, want := range []string{"packages: pass=1 fail=1", "pkg/fail", "TestOne", "panic: boom"} {
		if !strings.Contains(goJSON, want) {
			t.Fatalf("expected %q in go json summary:\n%s", want, goJSON)
		}
	}
	allPass := filters.SummarizeGoTestJSON(`{"Action":"pass","Package":"pkg/pass"}`)
	if allPass != "packages: pass=1 fail=0\nall tests passed" {
		t.Fatalf("unexpected all pass summary: %q", allPass)
	}
	goJSON = filters.SummarizeGoTestJSON(strings.Join([]string{
		`{"Action":"fail","Package":"pkg/fail","Test":"One"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Two"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Three"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Four"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Five"}`,
		`{"Action":"output","Package":"pkg/fail","Output":"not a panic"}`,
	}, "\n"))
	if !strings.Contains(goJSON, "... +1 more") {
		t.Fatalf("expected truncated go json failures: %q", goJSON)
	}
	compactFallback := filters.SummarizeGoTestJSON("not-json")
	if compactFallback != "not-json" {
		t.Fatalf("unexpected go json fallback: %q", compactFallback)
	}
}

func TestJSONStructure(t *testing.T) {
	t.Parallel()

	if got := filters.RenderJSONStructure([]byte("{bad")); got != "invalid json" {
		t.Fatalf("unexpected invalid json result: %q", got)
	}
	jsonShape := filters.RenderJSONStructure([]byte(`{"a":"x","b":1,"c":true,"d":null,"e":[{"z":"q"}]}`))
	for _, want := range []string{"a: string", "b: number", "c: bool", "d: null", "e: [", "z: string"} {
		if !strings.Contains(jsonShape, want) {
			t.Fatalf("expected %q in json shape:\n%s", want, jsonShape)
		}
	}
	if got := filters.RenderValueStructure(struct{}{}); got != "struct {}" {
		t.Fatalf("unexpected direct value render: %q", got)
	}
	if got := filters.RenderJSONStructure([]byte(`[]`)); got != "[]" {
		t.Fatalf("unexpected empty array shape: %q", got)
	}
}
