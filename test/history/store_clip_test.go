package history_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

// A command line has no length bound, so an unclipped record could exceed the
// limit its own reader will parse - the failure that made `szr spread` report
// "failed to read history: bufio.Scanner: token too long".
func TestAppendClipsOversizedCommandText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)

	command := "szr grep needle " + strings.Repeat("path/to/file ", 100_000)
	fingerprint := history.Fingerprint(command)
	if err := store.Append(history.Record{
		Timestamp:          time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		Command:            command,
		CommandFingerprint: fingerprint,
		Profile:            "grep",
		RawTokens:          100,
		FilteredTokens:     10,
	}); err != nil {
		t.Fatalf("append oversized command: %v", err)
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load after oversized command: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected the clipped record to be readable, got %d records", len(records))
	}
	rec := records[0]
	if len(rec.Command) >= len(command) {
		t.Fatalf("expected the command to be clipped, kept %d bytes", len(rec.Command))
	}
	if !strings.HasPrefix(rec.Command, "szr grep needle ") {
		t.Fatalf("expected the leading tokens to survive, got %q", rec.Command[:32])
	}
	if rec.CommandFingerprint != fingerprint {
		t.Fatalf("expected the full-command fingerprint to survive clipping, got %q", rec.CommandFingerprint)
	}
}

func TestAppendKeepsShortCommandTextExact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)

	if err := store.Append(history.Record{
		Timestamp: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		Command:   "git status --short",
		Profile:   "git-status",
	}); err != nil {
		t.Fatalf("append record: %v", err)
	}
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 1 || records[0].Command != "git status --short" {
		t.Fatalf("expected the command stored verbatim, got %#v", records)
	}
}
