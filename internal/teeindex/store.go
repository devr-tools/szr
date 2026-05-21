package teeindex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const indexFileName = "index.jsonl"

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
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, hydrateEntry(entry))
	}
	if err := scanner.Err(); err != nil {
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
