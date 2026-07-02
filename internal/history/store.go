package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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
	// compactMaxLineBytes is the longest single history line compaction will
	// tolerate; longer lines abort compaction and leave the file untouched.
	compactMaxLineBytes = 1 << 20
)

type Store struct {
	path          string
	maxFileBytes  int64
	retainRecords int
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
	if size > s.maxFileBytes {
		s.compact()
	}
	return nil
}

// compact rewrites the history file keeping only the most recent records.
// It is best-effort and crash-safe: retained lines are written to a temp file
// in the same directory and atomically renamed over the original, so a crash
// or error mid-compaction leaves the previous file intact.
func (s *Store) compact() {
	retained, ok := s.retainedLines()
	if !ok {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), "history-compact-*.tmp")
	if err != nil {
		return
	}
	if !writeCompactedLines(tmp, retained) {
		_ = os.Remove(tmp.Name())
		return
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		_ = os.Remove(tmp.Name())
	}
}

// retainedLines returns the newest history lines that fit within both the
// retained-record cap and half the size cap, so the compacted file has room
// to grow before the next compaction. It returns ok=false when compaction
// should be skipped (read error, oversized line, or nothing to drop).
func (s *Store) retainedLines() ([][]byte, bool) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	var lines [][]byte
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), compactMaxLineBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		lines = append(lines, append([]byte(nil), line...))
	}
	if scanner.Err() != nil {
		return nil, false
	}

	retainBytes := s.maxFileBytes / 2
	total := int64(0)
	start := len(lines)
	for start > 0 && len(lines)-start < s.retainRecords {
		lineBytes := int64(len(lines[start-1]) + 1)
		if start < len(lines) && total+lineBytes > retainBytes {
			break
		}
		total += lineBytes
		start--
	}
	if start == 0 {
		return nil, false
	}
	return lines[start:], true
}

func writeCompactedLines(file *os.File, lines [][]byte) bool {
	writer := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := writer.Write(line); err != nil {
			_ = file.Close()
			return false
		}
		if err := writer.WriteByte('\n'); err != nil {
			_ = file.Close()
			return false
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		return false
	}
	return file.Close() == nil
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

	var records []Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		records = append(records, hydrateRecord(rec))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *Store) Clear() error {
	file, err := os.OpenFile(s.path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
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
