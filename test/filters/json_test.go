package filters_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
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
	if allPass != "1 package passed" {
		t.Fatalf("unexpected all pass summary: %q", allPass)
	}
	multiPass := filters.SummarizeGoTestJSON(strings.Join([]string{
		`{"Action":"pass","Package":"pkg/a"}`,
		`{"Action":"pass","Package":"pkg/b"}`,
		`{"Action":"pass","Package":"pkg/c"}`,
	}, "\n"))
	if multiPass != "3 packages passed" {
		t.Fatalf("unexpected multi-package all pass summary: %q", multiPass)
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

// TestGoJSONSummaryAllPassStaysCheaperThanPlainOutput reproduces a
// benchmark loss: `szr go test` rewrites the command to `go test -json`, so
// the never-worse-than-raw guard compares the render against the JSON
// stream instead of the tiny plain output the user's command would have
// printed. The all-pass summary must therefore cost no more tokens than the
// plain-mode output — even for a single cached package.
func TestGoJSONSummaryAllPassStaysCheaperThanPlainOutput(t *testing.T) {
	t.Parallel()

	// Captured from `go test ./calc/ -run TestAdd -json` in the benchmark
	// arena repository; plain mode printed exactly one line.
	arenaJSON := strings.Join([]string{
		`{"Time":"2026-07-02T19:13:27.558747-04:00","Action":"start","Package":"arena/calc"}`,
		`{"Time":"2026-07-02T19:13:27.558834-04:00","Action":"run","Package":"arena/calc","Test":"TestAdd"}`,
		`{"Time":"2026-07-02T19:13:27.558849-04:00","Action":"output","Package":"arena/calc","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}`,
		`{"Time":"2026-07-02T19:13:27.558861-04:00","Action":"output","Package":"arena/calc","Test":"TestAdd","Output":"--- PASS: TestAdd (0.00s)\n"}`,
		`{"Time":"2026-07-02T19:13:27.558864-04:00","Action":"pass","Package":"arena/calc","Test":"TestAdd","Elapsed":0}`,
		`{"Time":"2026-07-02T19:13:27.558869-04:00","Action":"output","Package":"arena/calc","Output":"PASS\n"}`,
		`{"Time":"2026-07-02T19:13:27.558872-04:00","Action":"output","Package":"arena/calc","Output":"ok  \tarena/calc\t(cached)\n"}`,
		`{"Time":"2026-07-02T19:13:27.558876-04:00","Action":"pass","Package":"arena/calc","Elapsed":0}`,
	}, "\n")
	plainOutput := "ok  \tarena/calc\t(cached)"

	summary := filters.SummarizeGoTestJSON(arenaJSON)
	if got, limit := history.EstimateTokens(summary), history.EstimateTokens(plainOutput); got > limit {
		t.Fatalf("expected all-pass summary (%d tokens, %q) to cost no more than plain output (%d tokens, %q)", got, summary, limit, plainOutput)
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

	rendered := filters.RenderJSON([]byte(`{"a":"x","items":[{"id":1}]}`), filters.JSONModeStructure, 10)
	for _, want := range []string{"a: string", "id: number"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in json structure render:\n%s", want, rendered)
		}
	}
}

func TestJSONStructureLineCap(t *testing.T) {
	t.Parallel()

	// Structure mode now honors maxLines: only the first lines are rendered
	// and the remainder is folded into an overflow marker.
	capped := filters.RenderJSON([]byte(`{"a":"x","items":[{"id":1}]}`), filters.JSONModeStructure, 3)
	lines := strings.Split(capped, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 3 lines plus marker, got %d:\n%s", len(lines), capped)
	}
	if lines[len(lines)-1] != "... +5 more lines" {
		t.Fatalf("expected overflow marker, got %q:\n%s", lines[len(lines)-1], capped)
	}

	// A wide object is capped at the default budget when maxLines is 0.
	wide := "{"
	for i := 0; i < 100; i++ {
		if i > 0 {
			wide += ","
		}
		wide += `"zzz` + strconv.Itoa(i) + `":1`
	}
	wide += "}"
	rendered := filters.RenderJSONStructure([]byte(wide))
	got := strings.Split(rendered, "\n")
	if len(got) != 41 {
		t.Fatalf("expected default cap of 40 lines plus marker, got %d lines:\n%s", len(got), rendered)
	}
	// 100 keys + "{" + "}" = 102 total lines, 40 shown.
	if got[len(got)-1] != "... +62 more lines" {
		t.Fatalf("expected overflow marker for wide object, got %q", got[len(got)-1])
	}
}

func TestJSONStructureDepthCap(t *testing.T) {
	t.Parallel()

	deep := filters.RenderJSONStructure([]byte(`{"a":{"b":{"c":{"d":{"e":1,"f":2},"arr":[1,2,3]}}}}`))
	if !strings.Contains(deep, "d: {... 2 keys}") {
		t.Fatalf("expected depth-capped object summary in:\n%s", deep)
	}
	if !strings.Contains(deep, "arr: [3 items]") {
		t.Fatalf("expected depth-capped array summary in:\n%s", deep)
	}
	if strings.Contains(deep, "e: number") {
		t.Fatalf("expected nodes beyond depth cap to be summarized:\n%s", deep)
	}
}

func TestJSONStructureKeyPriority(t *testing.T) {
	t.Parallel()

	rendered := filters.RenderJSONStructure([]byte(`{"zeta":"later","message":"boom","status":"FAILED","id":"vm_123"}`))
	order := []string{"id: string", "status: string", "message: string", "zeta: string"}
	last := -1
	for _, want := range order {
		idx := strings.Index(rendered, want)
		if idx < 0 {
			t.Fatalf("expected %q in structure render:\n%s", want, rendered)
		}
		if idx <= last {
			t.Fatalf("expected %q after prior priority keys:\n%s", want, rendered)
		}
		last = idx
	}
}

func TestJSONStructureArrayItemCount(t *testing.T) {
	t.Parallel()

	rendered := filters.RenderJSONStructure([]byte(`{"items":[{"id":1},{"id":2},{"id":3}]}`))
	if !strings.Contains(rendered, "... +2 more items") {
		t.Fatalf("expected remaining item marker in:\n%s", rendered)
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
