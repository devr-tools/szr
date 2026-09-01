package jsonl_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/devr-tools/szr/internal/jsonl"
)

func collect(t *testing.T, input string, maxLineBytes int) ([]string, int) {
	t.Helper()
	var got []string
	skipped, err := jsonl.Scan(strings.NewReader(input), maxLineBytes, func(line []byte) {
		got = append(got, string(line))
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return got, skipped
}

func TestScanReadsLinesAndSkipsBlanks(t *testing.T) {
	got, skipped := collect(t, "a\n\nb\r\nc", 0)
	if skipped != 0 {
		t.Fatalf("expected no skips, got %d", skipped)
	}
	if strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("unexpected lines %q", got)
	}
}

func TestScanSkipsOversizedLinesAndKeepsGoing(t *testing.T) {
	long := strings.Repeat("x", 200*1024)
	got, skipped := collect(t, "first\n"+long+"\nlast\n", 64*1024)
	if skipped != 1 {
		t.Fatalf("expected 1 skipped line, got %d", skipped)
	}
	if strings.Join(got, ",") != "first,last" {
		t.Fatalf("expected the short lines to survive, got %q", got)
	}
}

func TestScanSkipsOversizedFinalLineWithoutNewline(t *testing.T) {
	got, skipped := collect(t, "first\n"+strings.Repeat("x", 200*1024), 64*1024)
	if skipped != 1 || len(got) != 1 || got[0] != "first" {
		t.Fatalf("unexpected result lines=%q skipped=%d", got, skipped)
	}
}

func TestScanAcceptsLineAtTheLimit(t *testing.T) {
	exact := strings.Repeat("x", 128*1024)
	got, skipped := collect(t, exact+"\n", len(exact))
	if skipped != 0 || len(got) != 1 || got[0] != exact {
		t.Fatalf("expected the limit-length line to be read, len=%d skipped=%d", len(got), skipped)
	}
}

func TestScanReturnsReadErrors(t *testing.T) {
	want := errors.New("boom")
	reader := io.MultiReader(strings.NewReader("ok\n"), &failingReader{err: want})
	seen := 0
	if _, err := jsonl.Scan(reader, 0, func([]byte) { seen++ }); !errors.Is(err, want) {
		t.Fatalf("expected the read error, got %v", err)
	}
	if seen != 1 {
		t.Fatalf("expected the line before the error, got %d", seen)
	}
}

type failingReader struct{ err error }

func (r *failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestClipBoundsTextAtRuneBoundary(t *testing.T) {
	if got := jsonl.Clip("git status", 64); got != "git status" {
		t.Fatalf("expected short text unchanged, got %q", got)
	}
	if got := jsonl.Clip("git status", 0); got != "git status" {
		t.Fatalf("expected a non-positive limit to disable clipping, got %q", got)
	}
	// A multi-byte rune straddling the limit must not be cut in half.
	clipped := jsonl.Clip(strings.Repeat("a", 7)+"日本語", 8)
	if !utf8.ValidString(clipped) {
		t.Fatalf("expected valid utf-8 after clipping, got %q", clipped)
	}
	if clipped != strings.Repeat("a", 7)+"…" {
		t.Fatalf("expected the partial rune dropped, got %q", clipped)
	}
}
