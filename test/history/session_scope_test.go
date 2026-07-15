package history_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func TestRecordSessionScopeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	record := history.Record{
		Timestamp:    time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC),
		Command:      "szr git status",
		Profile:      "git-status",
		SessionScope: "sess-abc",
	}
	if err := store.Append(record); err != nil {
		t.Fatalf("append: %v", err)
	}
	records, err := store.LoadAll()
	if err != nil || len(records) != 1 {
		t.Fatalf("load: %v records=%+v", err, records)
	}
	if records[0].SessionScope != "sess-abc" {
		t.Fatalf("expected session scope round trip, got %+v", records[0])
	}
}

func TestRecordDecodeToleratesMissingSessionScope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	line := `{"timestamp":"2026-07-10T10:00:00Z","command":"szr git status","profile":"git-status","cwd":"/work","duration_ms":10,"exit_code":0,"raw_bytes":100,"filtered_bytes":10,"raw_tokens":25,"filtered_tokens":3,"saved_tokens":22,"savings_pct":88}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy history: %v", err)
	}
	records, err := history.New(path).LoadAll()
	if err != nil || len(records) != 1 {
		t.Fatalf("load legacy: %v records=%+v", err, records)
	}
	if records[0].SessionScope != "" || records[0].Command != "szr git status" {
		t.Fatalf("expected empty scope on legacy record, got %+v", records[0])
	}
}
