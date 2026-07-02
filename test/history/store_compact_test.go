package history_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func TestAppendCompactsOversizedHistory(t *testing.T) {
	const total = 200
	const maxBytes = 8 * 1024
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, maxBytes, 10)

	for i := 0; i < total; i++ {
		if err := store.Append(compactionRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) == 0 || len(records) >= total {
		t.Fatalf("expected compaction to drop old records, got %d of %d", len(records), total)
	}
	assertNewestContiguousRecords(t, records, total)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if info.Size() > maxBytes {
		t.Fatalf("expected history file to stay within %d bytes, got %d", maxBytes, info.Size())
	}
}

func TestAppendCompactsWithDefaultLimits(t *testing.T) {
	const total = 2600
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)

	for i := 0; i < total; i++ {
		rec := compactionRecord(i)
		rec.Cwd = wideCompactionPayload
		if err := store.Append(rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) == 0 || len(records) >= total {
		t.Fatalf("expected default-limit compaction to drop old records, got %d of %d", len(records), total)
	}
	assertNewestContiguousRecords(t, records, total)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if info.Size() > history.DefaultCompactMaxBytes {
		t.Fatalf("expected history file to stay within %d bytes, got %d", history.DefaultCompactMaxBytes, info.Size())
	}
}

func TestAppendCompactionPreservesBudgetSuggestionLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 4*1024, 8)
	fingerprint := history.Fingerprint("szr go test ./...")

	for i := 0; i < 40; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 6, 1, 10, i, 0, 0, time.UTC),
			Command:            "szr go test ./...",
			CommandFingerprint: fingerprint,
			Profile:            "go-test-json",
			RawTokens:          120,
			FilteredTokens:     88,
			SavedTokens:        32,
			SavingsPct:         26.67,
			BytesEmitted:       360,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) >= 40 {
		t.Fatalf("expected compaction to shrink history, got %d records", len(records))
	}
	suggestion, err := store.FindBudgetSuggestion(fingerprint, history.BudgetSuggestionOptions{})
	if err != nil {
		t.Fatalf("find suggestion after compaction: %v", err)
	}
	if suggestion == nil || suggestion.Fingerprint != fingerprint {
		t.Fatalf("expected suggestion to survive compaction, got %#v", suggestion)
	}
}

func TestAppendKeepsSmallHistoryIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 64*1024, 10)

	for i := 0; i < 20; i++ {
		if err := store.Append(compactionRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 20 {
		t.Fatalf("expected small history to stay intact, got %d records", len(records))
	}
}

func TestAppendCompactionSkipsUnscannableHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 512, 4)

	oversized := make([]byte, 2<<20)
	for i := range oversized {
		oversized[i] = 'x'
	}
	if err := os.WriteFile(path, append(oversized, '\n'), 0o644); err != nil {
		t.Fatalf("seed oversized line: %v", err)
	}

	if err := store.Append(compactionRecord(0)); err != nil {
		t.Fatalf("append after oversized line: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if info.Size() < int64(len(oversized)) {
		t.Fatalf("expected compaction to leave unscannable history untouched, size %d", info.Size())
	}
}

func assertNewestContiguousRecords(t *testing.T, records []history.Record, total int) {
	t.Helper()
	start := total - len(records)
	for i, rec := range records {
		if want := compactionCommand(start + i); rec.Command != want {
			t.Fatalf("expected contiguous newest records, got %q at index %d, want %q", rec.Command, i, want)
		}
	}
}

func compactionRecord(i int) history.Record {
	return history.Record{
		Timestamp:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
		Command:        compactionCommand(i),
		Profile:        "git-status",
		DurationMS:     int64(10 + i%50),
		RawTokens:      100,
		FilteredTokens: 20,
		SavedTokens:    80,
		SavingsPct:     80,
	}
}

func compactionCommand(i int) string {
	return fmt.Sprintf("szr git status --run-%04d", i)
}

var wideCompactionPayload = func() string {
	payload := make([]byte, 900)
	for i := range payload {
		payload[i] = 'p'
	}
	return string(payload)
}()
