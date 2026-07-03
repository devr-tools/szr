package filters_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	fsfilter "github.com/devr-tools/szr/internal/filters/fs"
)

func buildServiceLog(cycles int, errorEvery int) string {
	messages := []string{
		"INFO worker starting batch cycle",
		"INFO fetched 40 records from queue",
		"DEBUG cache warm ratio 0.93",
		"INFO flushed batch to store",
		"WARN slow query detected on orders index",
	}
	var out strings.Builder
	for i := 0; i < cycles; i++ {
		ts := fmt.Sprintf("2026-07-03T10:%02d:%02d.%03dZ", (i/60)%60, i%60, (i*37)%1000)
		if errorEvery > 0 && i > 0 && i%errorEvery == 0 {
			out.WriteString(ts + " ERROR connection reset by peer while flushing batch\n")
			out.WriteString(ts + " ERROR retrying flush attempt 1/3\n")
			continue
		}
		out.WriteString(ts + " " + messages[i%len(messages)] + "\n")
	}
	return out.String()
}

func TestLooksLikeLogTextDetection(t *testing.T) {
	if !filters.LooksLikeLogText(buildServiceLog(60, 0)) {
		t.Fatalf("expected timestamped service log to be detected as log-shaped")
	}
	leveled := strings.Repeat("INFO listener ready\nWARN disk usage high\n", 6)
	if !filters.LooksLikeLogText(leveled) {
		t.Fatalf("expected level-prefixed log to be detected as log-shaped")
	}
	source := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\nvar x = 1\nconst y = 2\n"
	if filters.LooksLikeLogText(source) {
		t.Fatalf("expected Go source not to be detected as log-shaped")
	}
	markdown := "# Title\n\nSome prose about the project.\n- item one\n- item two\nMore prose here.\nAnother paragraph.\n"
	if filters.LooksLikeLogText(markdown) {
		t.Fatalf("expected markdown not to be detected as log-shaped")
	}
	if filters.LooksLikeLogText("2026-07-03T10:00:00Z INFO too short\n") {
		t.Fatalf("expected tiny inputs to skip log routing")
	}
}

func TestSummarizeLogTextHistogramAndDedup(t *testing.T) {
	input := buildServiceLog(200, 50)
	text, total := filters.SummarizeLogText(input, 12)
	if total != 203 {
		t.Fatalf("expected 203 total lines, got %d", total)
	}
	// 3 error pairs at i=50,100,150 -> 6 ERROR lines; remaining 194 follow the
	// 5-message cycle.
	histogram := strings.SplitN(text, "\n", 2)[0]
	if !strings.HasPrefix(histogram, "203 lines: ") {
		t.Fatalf("expected histogram first line, got %q", histogram)
	}
	for _, want := range []string{"INFO", "WARN", "DEBUG", "6 ERROR"} {
		if !strings.Contains(histogram, want) {
			t.Fatalf("expected %q in histogram %q", want, histogram)
		}
	}
	for _, want := range []string{
		"ERROR connection reset by peer while flushing batch (x3, ",
		"ERROR retrying flush attempt 1/3 (x3, ",
		"WARN slow query detected on orders index (x",
		"INFO worker starting batch cycle (x",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in summary:\n%s", want, text)
		}
	}
}

func TestSummarizeLogTextHistogramCounts(t *testing.T) {
	input := strings.Repeat("10:00:01 INFO a\n10:00:02 INFO a\n10:00:03 WARN b\n10:00:04 ERROR c\n", 10)
	text, total := filters.SummarizeLogText(input, 12)
	if total != 40 {
		t.Fatalf("expected 40 lines, got %d", total)
	}
	histogram := strings.SplitN(text, "\n", 2)[0]
	if histogram != "40 lines: 20 INFO, 10 ERROR, 10 WARN" {
		t.Fatalf("unexpected histogram: %q", histogram)
	}
}

func TestSummarizeLogTextThousandsSeparators(t *testing.T) {
	input := strings.Repeat("2026-07-03T10:00:00.000Z INFO worker starting batch cycle\n", 12000)
	text, total := filters.SummarizeLogText(input, 12)
	if total != 12000 {
		t.Fatalf("expected 12000 lines, got %d", total)
	}
	if !strings.HasPrefix(text, "12,000 lines: 12,000 INFO") {
		t.Fatalf("expected comma-separated histogram, got %q", strings.SplitN(text, "\n", 2)[0])
	}
}

func TestSummarizeLogTextErrorsNeverDropped(t *testing.T) {
	var out strings.Builder
	for i := 0; i < 8; i++ {
		out.WriteString(fmt.Sprintf("10:00:%02d ERROR failure mode %c occurred\n", i, 'A'+i))
	}
	out.WriteString(strings.Repeat("10:01:00 INFO steady state\n", 50))
	text, _ := filters.SummarizeLogText(out.String(), 3)
	for i := 0; i < 8; i++ {
		want := fmt.Sprintf("ERROR failure mode %c occurred", 'A'+i)
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q kept under tight budget:\n%s", want, text)
		}
	}
}

func TestSummarizeLogTextTimeRanges(t *testing.T) {
	input := strings.Join([]string{
		"2026-07-03T10:12:05.000Z ERROR connection reset by peer",
		"2026-07-03T10:31:10.000Z ERROR connection reset by peer",
		"2026-07-03T10:58:59.000Z ERROR connection reset by peer",
		"2026-07-03T10:12:06.000Z INFO worker ok",
		"2026-07-03T10:12:07.000Z INFO worker ok",
		"2026-07-03T10:20:00.000Z WARN single warning here",
	}, "\n")
	text, _ := filters.SummarizeLogText(input, 12)
	if !strings.Contains(text, "ERROR connection reset by peer (x3, 10:12:05–10:58:59)") {
		t.Fatalf("expected error count and first–last clock range:\n%s", text)
	}
	if !strings.Contains(text, "WARN single warning here (10:20:00)") {
		t.Fatalf("expected single-occurrence clock without count:\n%s", text)
	}
}

func TestSummarizeReadFileRoutesLogContent(t *testing.T) {
	logText := buildServiceLog(200, 50)
	rendered := fsfilter.SummarizeReadFile("logs/app.log", []byte(logText), 12)
	if !strings.Contains(rendered, "203 lines:") || !strings.Contains(rendered, "ERROR connection reset by peer while flushing batch") {
		t.Fatalf("expected log summary for .log file:\n%s", rendered)
	}

	// A .txt file with prose content keeps the document preview.
	doc := fsfilter.SummarizeReadFile("notes.txt", []byte("# Notes\n\nplain prose line\n- bullet\nmore prose\nfinal thought\n"), 6)
	if strings.Contains(doc, "lines:") {
		t.Fatalf("expected doc preview for prose .txt, got log summary:\n%s", doc)
	}

	// A .go file full of log-like lines still uses the code preview path.
	goSource := "package main\n\nfunc main() {\n\tprintln(\"x\")\n}\n"
	code := fsfilter.SummarizeReadFile("main.go", []byte(goSource), 6)
	if !strings.Contains(code, "package main") {
		t.Fatalf("expected code preview for Go file:\n%s", code)
	}
}

func TestLogAwareReadReducerStreamingEquivalence(t *testing.T) {
	logText := buildServiceLog(200, 50)
	direct := fsfilter.SummarizeReadFile("logs/app.log", []byte(logText), 12)

	reducer := newLogReadReducer(t, "logs/app.log", 12)
	feedInChunks(reducer, logText, 97)
	streamed := reducer.Result()
	if streamed != direct {
		t.Fatalf("streaming output diverged from buffered output:\n--- streamed ---\n%s\n--- direct ---\n%s", streamed, direct)
	}
	if kind, summary, ok := reducer.RecoveryInfo(); !ok || kind != "full-output" || !strings.Contains(summary, "summarized 203 log lines") {
		t.Fatalf("unexpected recovery info: %q %q %v", kind, summary, ok)
	}
}

func TestLogAwareReadReducerFallbackForNonLogText(t *testing.T) {
	doc := "# Notes\n\nplain prose line\n- bullet\nmore prose\nfinal thought\nclosing line\n"
	direct := fsfilter.SummarizeReadFile("notes.txt", []byte(doc), 6)

	reducer := newLogReadReducer(t, "notes.txt", 6)
	feedInChunks(reducer, doc, 11)
	if streamed := reducer.Result(); streamed != direct {
		t.Fatalf("fallback streaming output diverged:\n--- streamed ---\n%s\n--- direct ---\n%s", streamed, direct)
	}
}

func newLogReadReducer(t *testing.T, path string, maxLines int) *filters.LogAwareReadReducer {
	t.Helper()
	render := func(input string) string {
		return fsfilter.SummarizeReadFile(path, []byte(input), maxLines)
	}
	recovery := func(input string) (string, string, bool) {
		return fsfilter.ReadFileRecoveryInfo(path, []byte(input), maxLines)
	}
	return filters.NewLogAwareReadReducer(maxLines, render, recovery)
}

func feedInChunks(reducer *filters.LogAwareReadReducer, input string, chunkSize int) {
	data := []byte(input)
	for start := 0; start < len(data); start += chunkSize {
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		reducer.ConsumeStdout(data[start:end])
	}
}
