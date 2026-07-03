package filters_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
)

func streamingLogFixture(lines int) string {
	out := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		switch {
		case i%50 == 7:
			out = append(out, fmt.Sprintf("2026-07-02T10:%02d:%02dZ ERROR db connection refused", i/60%60, i%60))
		case i%9 == 3:
			out = append(out, fmt.Sprintf("2026-07-02T10:%02d:%02dZ WARN retrying request", i/60%60, i%60))
		default:
			out = append(out, fmt.Sprintf("2026-07-02T10:%02d:%02dZ INFO handled request path=/api/%d", i/60%60, i%60, i%4))
		}
	}
	return strings.Join(out, "\n") + "\n"
}

// The streaming summarizer must render exactly what the batch summarizer
// renders, even when chunks split lines (and multi-byte content) at
// arbitrary byte offsets.
func TestStreamingLogSummarizerMatchesBatchRender(t *testing.T) {
	t.Parallel()

	input := streamingLogFixture(400)
	want, wantTotal := filters.SummarizeLogText(input, 12)

	stream := filters.NewStreamingLogSummarizer()
	data := []byte(input)
	for start := 0; start < len(data); start += 7 {
		end := start + 7
		if end > len(data) {
			end = len(data)
		}
		stream.Consume(data[start:end])
	}

	got := stream.Result(12)
	if got != want {
		t.Fatalf("streaming render diverged from batch render:\n--- streaming ---\n%s\n--- batch ---\n%s", got, want)
	}
	if stream.TotalLines() != wantTotal {
		t.Fatalf("expected %d total lines, got %d", wantTotal, stream.TotalLines())
	}
}

func TestStreamingLogSummarizerKeepsErrorsAndHistogram(t *testing.T) {
	t.Parallel()

	stream := filters.NewStreamingLogSummarizer()
	stream.Consume([]byte(streamingLogFixture(300)))
	got := stream.Result(10)

	if !strings.Contains(got, "lines:") {
		t.Fatalf("expected severity histogram line, got:\n%s", got)
	}
	if !strings.Contains(got, "ERROR db connection refused") {
		t.Fatalf("expected distinct error message to survive, got:\n%s", got)
	}
	if stream.DistinctMessages() == 0 {
		t.Fatal("expected distinct message accounting")
	}
}

// A final line without a trailing newline must still be counted once Result
// flushes the scanner, and Result must be idempotent.
func TestStreamingLogSummarizerFlushesPartialFinalLine(t *testing.T) {
	t.Parallel()

	stream := filters.NewStreamingLogSummarizer()
	stream.Consume([]byte("2026-07-02T10:00:01Z INFO first\n2026-07-02T10:00:02Z ERROR last without newline"))
	first := stream.Result(8)
	if stream.TotalLines() != 2 {
		t.Fatalf("expected 2 ingested lines, got %d", stream.TotalLines())
	}
	if !strings.Contains(first, "ERROR last without newline") {
		t.Fatalf("expected flushed final line in render, got:\n%s", first)
	}
	if second := stream.Result(8); second != first {
		t.Fatalf("expected idempotent Result, got:\n%s\nvs\n%s", second, first)
	}
}
