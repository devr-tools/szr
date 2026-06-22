package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
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

	rendered := filters.RenderJSON([]byte(`{"a":"x","items":[{"id":1}]}`), filters.JSONModeStructure, 3)
	for _, want := range []string{"a: string", "id: number"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in json structure render:\n%s", want, rendered)
		}
	}
}

func TestJSONPreview(t *testing.T) {
	t.Parallel()

	if got := filters.SummarizeJSONPreview([]byte("{bad"), 6); got != "invalid json" {
		t.Fatalf("unexpected invalid preview result: %q", got)
	}

	preview := filters.SummarizeJSONPreview([]byte(`{"user":{"id":7,"name":"alex"},"items":[{"name":"alpha","count":1},{"name":"beta","count":2}],"ok":true}`), 8)
	for _, want := range []string{
		"root: object keys=3",
		"user: object keys=2",
		"user.id=7",
		`user.name="alex"`,
		`items: array len=2 sample=object{name,count}, object{name,count}`,
		"ok=true",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected %q in json preview:\n%s", want, preview)
		}
	}

	rootArray := filters.SummarizeJSONPreview([]byte(`[{"id":"a1","ok":true},{"id":"a2","ok":false}]`), 5)
	for _, want := range []string{
		"root: array len=2",
		"sample=object{id,ok}, object{id,ok}",
		"root[0]: object keys=2",
		`root[0].id="a1"`,
	} {
		if !strings.Contains(rootArray, want) {
			t.Fatalf("expected %q in root array preview:\n%s", want, rootArray)
		}
	}

	prioritized := filters.SummarizeJSONPreview([]byte(`{"zeta":"later","message":"boom","updated_at":"2026-05-25T10:00:00Z","status":"FAILED","name":"api","id":"vm_123"}`), 5)
	order := []string{
		`id="vm_123"`,
		`name="api"`,
		`status="FAILED"`,
		`updated_at="2026-05-25T10:00:00Z"`,
	}
	last := -1
	for _, want := range order {
		idx := strings.Index(prioritized, want)
		if idx < 0 {
			t.Fatalf("expected %q in prioritized preview:\n%s", want, prioritized)
		}
		if idx <= last {
			t.Fatalf("expected %q after prior priority fields in preview:\n%s", want, prioritized)
		}
		last = idx
	}

	rendered := filters.RenderJSON([]byte(`{"user":{"id":7,"name":"alex"},"items":[{"name":"alpha"}],"ok":true}`), filters.JSONModePreview, 6)
	for _, want := range []string{"root: object keys=3", "items: array len=1 sample=object{name}", "user: object keys=2", "user.id=7"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in rendered json preview:\n%s", want, rendered)
		}
	}
}

func TestRenderJSONModeValidation(t *testing.T) {
	t.Parallel()

	if got := filters.RenderJSON([]byte(`{"ok":true}`), "detail", 4); got != "unsupported json mode: detail" {
		t.Fatalf("unexpected unsupported mode response: %q", got)
	}
}
