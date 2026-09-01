package history_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

// oversizedRecordLine is a valid record whose command alone pushes the line
// past what readers parse - the shape a pre-clipping szr wrote for a command
// with a very long argument list.
func oversizedRecordLine(t *testing.T) []byte {
	t.Helper()
	line, err := json.Marshal(history.Record{
		Timestamp:      time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC),
		Command:        "szr grep needle " + strings.Repeat("path/to/file ", 120_000),
		Profile:        "grep",
		DurationMS:     1_200,
		RawTokens:      90_000,
		FilteredTokens: 900,
		SavedTokens:    89_100,
		SavingsPct:     99,
	})
	if err != nil {
		t.Fatalf("marshal oversized record: %v", err)
	}
	if len(line) <= 1<<20 {
		t.Fatalf("expected the fixture line to exceed the reader limit, got %d bytes", len(line))
	}
	return line
}

func TestLoadAllRecoversOversizedRecordMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	if err := os.WriteFile(path, append(oversizedRecordLine(t), '\n'), 0o644); err != nil {
		t.Fatalf("seed oversized record: %v", err)
	}

	records, err := history.New(path).LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected the oversized record to be recovered, got %d records", len(records))
	}
	rec := records[0]
	if rec.SavedTokens != 89_100 || rec.RawTokens != 90_000 || rec.DurationMS != 1_200 {
		t.Fatalf("expected the measurements to survive recovery, got %#v", rec)
	}
	if len(rec.Command) > 16*1024 || !strings.HasPrefix(rec.Command, "szr grep needle ") {
		t.Fatalf("expected the command clipped to its leading tokens, got %d bytes", len(rec.Command))
	}
}

func TestCompactionRewritesOversizedRecordClipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	line := oversizedRecordLine(t)
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("seed oversized record: %v", err)
	}

	// Any append triggers compaction here, since the seeded line already
	// exceeds the size cap.
	store := history.NewWithLimits(path, 64*1024, 100)
	if err := store.Append(compactionRecord(1)); err != nil {
		t.Fatalf("append: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if info.Size() >= int64(len(line)) {
		t.Fatalf("expected compaction to shrink the repaired record, size %d", info.Size())
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected the repaired record to be kept alongside the new one, got %d", len(records))
	}
	if records[0].SavedTokens != 89_100 {
		t.Fatalf("expected repair to keep the archived measurements, got %#v", records[0])
	}
	// Repair keeps the record in the file, so nothing is archived or dropped.
	if totals := store.Totals(); !totals.Empty() {
		t.Fatalf("expected a repaired record to stay a record, got totals %#v", totals)
	}
}

func TestCompactionCountsUnparseableLinesAsDropped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	torn := strings.Repeat("x", 2<<20)
	if err := os.WriteFile(path, []byte(torn+"\n"), 0o644); err != nil {
		t.Fatalf("seed torn line: %v", err)
	}

	store := history.NewWithLimits(path, 64*1024, 100)
	if err := store.Append(compactionRecord(1)); err != nil {
		t.Fatalf("append: %v", err)
	}

	totals := store.Totals()
	if totals.DroppedRecords != 1 {
		t.Fatalf("expected the unparseable line to be recorded as dropped, got %#v", totals)
	}
	if totals.Commands != 0 {
		t.Fatalf("expected no runs to be archived, got %d", totals.Commands)
	}
}

// An oversized line is re-parsed by every command that reads history, so a
// read must schedule its repair instead of waiting for the file to grow past
// the size trigger.
func TestReadSchedulesRepairBelowSizeTrigger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	line := oversizedRecordLine(t)
	if err := os.WriteFile(path, append(line, '\n'), 0o644); err != nil {
		t.Fatalf("seed oversized record: %v", err)
	}

	// A cap far above the seeded file: only the pending repair can trigger
	// compaction here.
	store := history.NewWithLimits(path, 64<<20, 2500)
	if _, err := store.LoadAll(); err != nil {
		t.Fatalf("load all: %v", err)
	}
	if err := store.Append(compactionRecord(1)); err != nil {
		t.Fatalf("append: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat history: %v", err)
	}
	if info.Size() >= int64(len(line)) {
		t.Fatalf("expected the read to schedule a repair, size %d", info.Size())
	}
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load after repair: %v", err)
	}
	if len(records) != 2 || records[0].SavedTokens != 89_100 {
		t.Fatalf("expected the repaired record to survive with its metrics, got %#v", records)
	}
}

func TestCleanHistoryDoesNotTriggerRepairCompaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 64<<20, 2500)
	for i := 0; i < 5; i++ {
		if err := store.Append(compactionRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		if _, err := store.LoadAll(); err != nil {
			t.Fatalf("load all: %v", err)
		}
	}
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("expected a clean history to be left alone, got %d records", len(records))
	}
	if totals := store.Totals(); !totals.Empty() {
		t.Fatalf("expected no archiving without compaction, got %#v", totals)
	}
}
