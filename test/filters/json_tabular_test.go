package filters_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	filters "github.com/devr-tools/szr/internal/filters"
)

func uniformItems(t *testing.T, count int, mutate func(i int, item map[string]any)) []any {
	t.Helper()
	items := make([]any, 0, count)
	for i := 0; i < count; i++ {
		item := map[string]any{
			"id":     float64(9000 + i),
			"title":  fmt.Sprintf("Improve retry backoff for connector %d", i+1),
			"state":  "open",
			"author": "alice",
		}
		if mutate != nil {
			mutate(i, item)
		}
		items = append(items, item)
	}
	return items
}

func TestSummarizeUniformJSONArrayBasic(t *testing.T) {
	t.Parallel()

	rendered, ok := filters.SummarizeUniformJSONArray("array", uniformItems(t, 6, nil), 12)
	if !ok {
		t.Fatal("expected tabular render for uniform array")
	}
	lines := strings.Split(rendered, "\n")
	if lines[0] != "array len=6 uniform objects, cols: id|title|state|author" {
		t.Fatalf("unexpected header: %q", lines[0])
	}
	if len(lines) != 7 {
		t.Fatalf("expected header plus 6 rows, got %d lines:\n%s", len(lines), rendered)
	}
	if lines[1] != "9000|Improve retry backoff for connector 1|open|alice" {
		t.Fatalf("unexpected first row: %q", lines[1])
	}
	if strings.Contains(rendered, "more rows") {
		t.Fatalf("expected no omission marker when all rows fit:\n%s", rendered)
	}
}

func TestSummarizeUniformJSONArrayAnomalyRowsSurvive(t *testing.T) {
	t.Parallel()

	items := uniformItems(t, 300, func(i int, item map[string]any) {
		if i == 217 {
			item["state"] = "spam_flagged"
			item["title"] = "PAY-4471 duplicate charge on retry | needs finance review"
		}
	})
	rendered, ok := filters.SummarizeUniformJSONArray("array", items, 12)
	if !ok {
		t.Fatal("expected tabular render")
	}
	for _, want := range []string{
		"array len=300 uniform objects",
		`#217 9217|"PAY-4471 duplicate charge on retry | needs finance review"|spam_flagged|alice`,
		"... +290 more rows",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in tabular render:\n%s", want, rendered)
		}
	}
	if got := len(strings.Split(rendered, "\n")); got > 12 {
		t.Fatalf("expected at most 12 lines, got %d", got)
	}
}

func TestSummarizeUniformJSONArrayNonUniformItemsAlwaysKept(t *testing.T) {
	t.Parallel()

	items := uniformItems(t, 200, func(i int, item map[string]any) {
		if i == 143 {
			item["message"] = "upstream timeout after 15000ms"
		}
	})
	rendered, ok := filters.SummarizeUniformJSONArray("stream", items, 12)
	if !ok {
		t.Fatal("expected tabular render")
	}
	if !strings.HasPrefix(rendered, "stream len=200 uniform objects") {
		t.Fatalf("unexpected header:\n%s", rendered)
	}
	if !strings.Contains(rendered, `#143 {id=9143 title="Improve retry backoff for connector 144" state="open" message="upstream timeout after 15000ms" author="alice"}`) {
		t.Fatalf("expected non-uniform row inline preview:\n%s", rendered)
	}
}

func TestSummarizeUniformJSONArrayCellEncoding(t *testing.T) {
	t.Parallel()

	items := uniformItems(t, 6, func(i int, item map[string]any) {
		switch i {
		case 1:
			item["title"] = "a|b"   // delimiter must be quoted
			item["state"] = nil     // bare null
			item["author"] = "null" // string "null" must stay distinct
		case 2:
			item["title"] = ""    // empty string
			item["state"] = "123" // number-shaped string
			item["author"] = map[string]any{"login": "alice", "bot": false}
		case 3:
			item["state"] = []any{1.0, 2.0}
			item["author"] = " padded "
		}
	})
	rendered, ok := filters.SummarizeUniformJSONArray("array", items, 20)
	if !ok {
		t.Fatal("expected tabular render")
	}
	for _, want := range []string{
		`9001|"a|b"|null|"null"`,
		`9002|""|"123"|{bot,login}`,
		`9003|Improve retry backoff for connector 4|[2 items]|" padded "`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in tabular render:\n%s", want, rendered)
		}
	}
}

func TestSummarizeUniformJSONArrayRejectsNonUniformShapes(t *testing.T) {
	t.Parallel()

	if _, ok := filters.SummarizeUniformJSONArray("array", uniformItems(t, 4, nil), 12); ok {
		t.Fatal("expected small arrays to keep the existing render")
	}

	mixed := make([]any, 0, 40)
	for i := 0; i < 40; i++ {
		switch i % 3 {
		case 0:
			mixed = append(mixed, map[string]any{"type": "user", "id": float64(i), "email": "a@b.c"})
		case 1:
			mixed = append(mixed, map[string]any{"type": "repo", "id": float64(i), "stars": float64(i)})
		default:
			mixed = append(mixed, map[string]any{"type": "event", "id": float64(i), "action": "push", "ts": "2026-06-01T08:00:00Z"})
		}
	}
	if _, ok := filters.SummarizeUniformJSONArray("array", mixed, 12); ok {
		t.Fatal("expected mixed-shape array to keep the existing render")
	}

	scalars := []any{1.0, 2.0, 3.0, 4.0, 5.0, 6.0}
	if _, ok := filters.SummarizeUniformJSONArray("array", scalars, 12); ok {
		t.Fatal("expected scalar array to keep the existing render")
	}
}

func TestSummarizeUniformJSONArrayColumnCap(t *testing.T) {
	t.Parallel()

	items := make([]any, 0, 6)
	for i := 0; i < 6; i++ {
		item := map[string]any{"id": float64(i), "status": "ok"}
		for c := 0; c < 12; c++ {
			item[fmt.Sprintf("metric_%02d", c)] = float64(c)
		}
		items = append(items, item)
	}
	rendered, ok := filters.SummarizeUniformJSONArray("array", items, 12)
	if !ok {
		t.Fatal("expected tabular render")
	}
	header := strings.Split(rendered, "\n")[0]
	if !strings.Contains(header, "(+4 more cols)") {
		t.Fatalf("expected column-cap marker in header: %q", header)
	}
	if got := strings.Count(strings.Split(rendered, "\n")[1], "|"); got != 9 {
		t.Fatalf("expected 10 cells per row, got %d delimiters", got)
	}
}

func TestSummarizeUniformJSONArrayFitsRawJSON(t *testing.T) {
	t.Parallel()

	// End-to-end sanity: marshal/unmarshal round trip mirrors the real
	// jsonquery input shape.
	items := uniformItems(t, 10, nil)
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	rendered, ok := filters.SummarizeUniformJSONArray("array", decoded, 20)
	if !ok {
		t.Fatal("expected tabular render")
	}
	if !strings.Contains(rendered, "9009|Improve retry backoff for connector 10|open|alice") {
		t.Fatalf("expected last row rendered:\n%s", rendered)
	}
}
