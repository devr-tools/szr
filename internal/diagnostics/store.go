package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
)

// Store is an append-only local JSONL event stream. Files are owner-readable
// only because diagnostic metadata can still reveal operational patterns.
type Store struct {
	path     string
	exporter *Exporter
}

func New(path string) *Store { return &Store{path: path} }

// NewWithExporter keeps local events durable even when remote export is
// disabled or unavailable. Export is queued only after the local append.
func NewWithExporter(path string, exporter *Exporter) *Store {
	return &Store{path: path, exporter: exporter}
}

func (s *Store) Append(event Event) error {
	if s == nil || s.path == "" {
		return nil
	}
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(event)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr == nil && s.exporter != nil {
		s.exporter.Enqueue(event)
	}
	return closeErr
}

// Flush asks the optional exporter to send pending events. It is intended for
// an explicit administrative action, never the command execution hot path.
// The boolean reports whether export was configured and available.
func (s *Store) Flush(ctx context.Context) (bool, error) {
	if s == nil || s.exporter == nil {
		return false, nil
	}
	return true, s.exporter.Flush(ctx)
}

// ReadAll returns valid events in file order. A malformed line is ignored so a
// partial write never prevents a local consumer from reading later events.
//
//nolint:maintidx // Reads tolerate malformed lines while preserving later events.
func (s *Store) ReadAll() ([]Event, error) {
	if s == nil || s.path == "" {
		return nil, nil
	}
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return events, nil
}
