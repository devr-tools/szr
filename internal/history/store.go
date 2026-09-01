package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/devr-tools/szr/internal/jsonl"
)

// History compaction keeps history.jsonl bounded. Design choice: rather than
// tail-reading the file or maintaining a separate suggestion cache, Append
// caps the on-disk size so that LoadAll — and therefore the per-command
// FindBudgetSuggestion lookup performed by the adaptive budget adapter —
// stays cheap. Scanning a <=2MiB JSONL file takes on the order of 10ms even
// near the cap, which bounds per-command overhead without any extra state
// that could drift out of sync with the file.
const (
	// DefaultCompactMaxBytes is the file size that triggers compaction on Append.
	DefaultCompactMaxBytes = 2 << 20
	// DefaultCompactRetainRecords caps how many of the most recent records a
	// compaction pass keeps.
	DefaultCompactRetainRecords = 2500
	// maxRecordLineBytes is the longest single history line readers will
	// parse. Longer lines are dropped rather than failing the read, and
	// compaction rewrites the file without them so it stops paying for
	// records nothing can use.
	maxRecordLineBytes = 1 << 20
	// maxCommandBytes bounds the command text stored per record. Command
	// lines are unbounded in practice - an agent can run a grep over
	// hundreds of paths - while readers only ever display the first few
	// tokens, so clipping on write keeps records well under
	// maxRecordLineBytes.
	maxCommandBytes = 8 << 10
)

type Store struct {
	path          string
	maxFileBytes  int64
	retainRecords int
	// repairPending records that a read in this process found a line needing
	// repair, so the next Append compacts even below the size trigger.
	// Without it an oversized line would sit in the file - and be re-parsed
	// by every command that reads history - until the file happened to cross
	// maxFileBytes.
	repairPending atomic.Bool
}

func New(path string) *Store {
	return NewWithLimits(path, DefaultCompactMaxBytes, DefaultCompactRetainRecords)
}

// NewWithLimits builds a store with custom compaction limits. Non-positive
// values fall back to the defaults. maxFileBytes is the append-time size
// trigger; retainRecords caps how many recent records survive compaction.
func NewWithLimits(path string, maxFileBytes int64, retainRecords int) *Store {
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultCompactMaxBytes
	}
	if retainRecords <= 0 {
		retainRecords = DefaultCompactRetainRecords
	}
	return &Store{path: path, maxFileBytes: maxFileBytes, retainRecords: retainRecords}
}

func (s *Store) Append(record Record) error {
	record.Command = jsonl.Clip(record.Command, maxCommandBytes)
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	appendErr := json.NewEncoder(file).Encode(record)
	size := int64(-1)
	if info, statErr := file.Stat(); statErr == nil {
		size = info.Size()
	}
	if closeErr := file.Close(); appendErr == nil {
		appendErr = closeErr
	}
	if appendErr != nil {
		return appendErr
	}
	if size > s.maxFileBytes || s.repairPending.Load() {
		s.compact()
	}
	return nil
}

// compact rewrites the history file keeping only the most recent records, and
// folds everything it removes into the archived totals so lifetime reporting
// stays whole. It is best-effort and crash-safe: retained lines are written to
// a temp file in the same directory and atomically renamed over the original,
// so a crash or error mid-compaction leaves the previous file intact.
func (s *Store) compact() {
	// Clear first: a failed pass leaves the line in place, and the next read
	// flags it again, so a broken rewrite retries rather than looping here.
	s.repairPending.Store(false)
	lines, stats, ok := s.readHistoryLines()
	if !ok {
		return
	}
	start := retainStart(lines, s.retainRecords, s.maxFileBytes/2)
	// Repaired and dropped lines are reason enough to rewrite: the file is
	// carrying bytes no reader can use, and leaving them means never
	// compacting again.
	if start == 0 && stats.repaired == 0 && stats.dropped == 0 {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "history-compact-*.tmp")
	if err != nil {
		return
	}
	if !writeCompactedLines(tmp, lines[start:]) {
		_ = os.Remove(tmp.Name())
		return
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		_ = os.Remove(tmp.Name())
		return
	}
	s.archive(archivedRecords(lines[:start]), stats.dropped, time.Now())
}

// archivedRecords collects the records compaction dropped from the file.
func archivedRecords(lines []historyLine) []Record {
	records := make([]Record, 0, len(lines))
	for _, line := range lines {
		records = append(records, line.record)
	}
	return records
}

func (s *Store) LoadAll() ([]Record, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	// An oversized record still reports real measurements, so it counts here
	// with its command clipped, and the read schedules a compaction to
	// persist the clipped form.
	var records []Record
	repaired := 0
	if _, err := jsonl.Scan(file, repairMaxLineBytes, func(line []byte) {
		rec, oversized, ok := decodeRecord(line)
		if !ok {
			return
		}
		if oversized {
			repaired++
		}
		records = append(records, hydrateRecord(rec))
	}); err != nil {
		return nil, err
	}
	if repaired > 0 {
		s.repairPending.Store(true)
	}
	return records, nil
}

// Clear empties the history file and discards the archived totals, so a
// cleared store reports nothing rather than lifetime counters with no records
// behind them.
func (s *Store) Clear() error {
	file, err := os.OpenFile(s.path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := os.Remove(s.totalsPath()); err != nil && !os.IsNotExist(err) {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (s *Store) SuggestBudgets(opts BudgetSuggestionOptions) ([]BudgetSuggestion, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return SuggestBudgets(records, opts), nil
}

// FindBudgetSuggestion loads the full history for the lookup; this is safe to
// call once per command because Append-time compaction keeps the file (and
// therefore LoadAll) bounded.
func (s *Store) FindBudgetSuggestion(fingerprint string, opts BudgetSuggestionOptions) (*BudgetSuggestion, error) {
	records, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return FindBudgetSuggestion(records, fingerprint, opts), nil
}
