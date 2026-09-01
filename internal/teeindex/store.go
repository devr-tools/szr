package teeindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/jsonl"
)

const indexFileName = "index.jsonl"

const (
	// maxEntryLineBytes is the longest single index line readers will parse;
	// longer lines are dropped instead of failing the whole read.
	maxEntryLineBytes = 1 << 20
	// maxCommandBytes bounds the command text stored per entry so an
	// unbounded command line cannot produce an entry readers must drop.
	maxCommandBytes = 8 << 10
)

type Entry struct {
	ID                 string    `json:"id"`
	Timestamp          time.Time `json:"timestamp"`
	Path               string    `json:"path"`
	Command            string    `json:"command"`
	CommandFingerprint string    `json:"command_fingerprint,omitempty"`
	Profile            string    `json:"profile"`
	ProfileConfidence  string    `json:"profile_confidence,omitempty"`
	Cwd                string    `json:"cwd,omitempty"`
	ExitCode           int       `json:"exit_code"`
	DurationMS         int64     `json:"duration_ms,omitempty"`
	RawBytes           int       `json:"raw_bytes,omitempty"`
	RawTokens          int       `json:"raw_tokens,omitempty"`
}

type Store struct {
	dir  string
	path string
}

func New(dir string) *Store {
	return &Store{
		dir:  dir,
		path: filepath.Join(dir, indexFileName),
	}
}

func (s *Store) Append(entry Entry) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("tee index store is not configured")
	}
	if entry.ID == "" {
		entry.ID = idForPath(entry.Path)
	}
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	entry.Command = jsonl.Clip(entry.Command, maxCommandBytes)
	enc := json.NewEncoder(file)
	return enc.Encode(entry)
}

func (s *Store) LoadAll() ([]Entry, error) {
	if s == nil || s.path == "" {
		return nil, nil
	}
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []Entry
	if _, err := jsonl.Scan(file, maxEntryLineBytes, func(line []byte) {
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			return
		}
		entries = append(entries, hydrateEntry(entry))
	}); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *Store) List(limit int) ([]Entry, error) {
	entries, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}
	return entries, nil
}

func (s *Store) Replace(entries []Entry) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("tee index store is not configured")
	}
	file, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	for _, entry := range entries {
		if err := enc.Encode(hydrateEntry(entry)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Latest() (Entry, bool, error) {
	entries, err := s.List(1)
	if err != nil {
		return Entry{}, false, err
	}
	if len(entries) == 0 {
		return Entry{}, false, nil
	}
	return entries[0], true, nil
}

func (s *Store) Find(id string) (Entry, bool, error) {
	entries, err := s.LoadAll()
	if err != nil {
		return Entry{}, false, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Entry{}, false, nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
	for _, entry := range entries {
		if entry.ID == id || filepath.Base(entry.Path) == id {
			return entry, true, nil
		}
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.ID, id) {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

func (s *Store) Read(entry Entry) ([]byte, error) {
	return os.ReadFile(entry.Path)
}

func (s *Store) Search(query string, limit int) ([]Entry, error) {
	entries, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return s.List(limit)
	}

	matches := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		hydrated := hydrateEntry(entry)
		haystack := strings.ToLower(strings.Join([]string{
			hydrated.ID,
			hydrated.Path,
			hydrated.Command,
			hydrated.Profile,
			hydrated.CommandFingerprint,
		}, "\n"))
		if strings.Contains(haystack, query) {
			matches = append(matches, hydrated)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Timestamp.After(matches[j].Timestamp)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func hydrateEntry(entry Entry) Entry {
	if entry.ID == "" {
		entry.ID = idForPath(entry.Path)
	}
	return entry
}

func idForPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
