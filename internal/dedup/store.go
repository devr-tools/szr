// Package dedup persists the session dedup index and its raw-output
// artifacts. Each filtered run that is large enough to be worth referencing
// appends one entry keyed by the full SHA-256 of the raw output plus the
// command fingerprint, working directory, and exit code. A later identical
// run within the recency window is rendered as a short reference; `szr
// expand <ref>` recovers the stored raw bytes.
package dedup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DirName is the subdirectory of the szr data dir that holds the dedup
	// index and its raw artifacts.
	DirName       = "dedup"
	indexFileName = "index.jsonl"
	// RefLength is how many leading hex characters of the raw hash form the
	// displayed reference.
	RefLength = 12
	// MinRefLength keeps prefix lookups from matching on a token too short
	// to identify anything.
	MinRefLength = 4
	// compactMaxIndexBytes triggers append-time compaction, mirroring the
	// history store's size-capped pattern so LoadAll stays cheap.
	compactMaxIndexBytes = 1 << 20
	// compactRetainEntries caps how many recent entries a compaction keeps.
	compactRetainEntries = 1500
)

// Entry records one run eligible for session dedup. RawHash is the full
// SHA-256 of the raw output (the dedup key); ArtifactHash is the SHA-256 of
// the stored artifact bytes, which differs from RawHash only when the
// artifact was truncated at the storage cap.
type Entry struct {
	Timestamp          time.Time `json:"timestamp"`
	RawHash            string    `json:"raw_hash"`
	ArtifactHash       string    `json:"artifact_hash"`
	ArtifactPath       string    `json:"artifact_path"`
	Command            string    `json:"command"`
	CommandFingerprint string    `json:"command_fingerprint"`
	Cwd                string    `json:"cwd,omitempty"`
	ExitCode           int       `json:"exit_code"`
	RawBytes           int64     `json:"raw_bytes,omitempty"`
	Truncated          bool      `json:"truncated,omitempty"`
}

// Ref returns the short reference displayed to the caller.
func (e Entry) Ref() string {
	if len(e.RawHash) <= RefLength {
		return e.RawHash
	}
	return e.RawHash[:RefLength]
}

// Key identifies the honesty boundary for a dedup match: the same command
// text run in the same directory with the same exit code and byte-identical
// raw output.
type Key struct {
	CommandFingerprint string
	Cwd                string
	ExitCode           int
	RawHash            string
}

func (k Key) matches(e Entry) bool {
	return e.CommandFingerprint == k.CommandFingerprint &&
		e.Cwd == k.Cwd &&
		e.ExitCode == k.ExitCode &&
		e.RawHash == k.RawHash
}

type Store struct {
	dir  string
	path string
}

// New builds a store rooted at <dataDir>/dedup. A blank dataDir yields an
// unconfigured store whose operations fail or report nothing.
func New(dataDir string) *Store {
	if dataDir == "" {
		return &Store{}
	}
	dir := filepath.Join(dataDir, DirName)
	return &Store{dir: dir, path: filepath.Join(dir, indexFileName)}
}

// Append writes one entry as a single JSONL line. The index is append-only
// so concurrent szr processes can write without coordination; readers
// tolerate the partial lines a race can leave behind.
func (s *Store) Append(entry Entry) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("dedup store is not configured")
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	size, err := appendIndexLine(s.path, append(line, '\n'))
	if err != nil {
		return err
	}
	if size > compactMaxIndexBytes {
		s.compact()
	}
	return nil
}

func appendIndexLine(path string, line []byte) (int64, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	_, writeErr := file.Write(line)
	size := int64(-1)
	if info, statErr := file.Stat(); statErr == nil {
		size = info.Size()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	return size, writeErr
}

// LoadAll reads every parseable entry. Unparseable lines (torn writes from
// concurrent appends) are skipped, mirroring the history store.
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
	return scanIndexEntries(file)
}

func scanIndexEntries(reader io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), compactMaxIndexBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Matches returns the entries for key recorded at or after since, newest
// first.
func (s *Store) Matches(key Key, since time.Time) ([]Entry, error) {
	entries, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	matches := make([]Entry, 0, 4)
	for _, entry := range entries {
		if key.matches(entry) && !entry.Timestamp.Before(since) {
			matches = append(matches, entry)
		}
	}
	sortNewestFirst(matches)
	return matches, nil
}

// FindRef resolves a reference by prefix match on the raw hash, preferring
// the newest entry. References shorter than MinRefLength are rejected.
func (s *Store) FindRef(ref string) (Entry, bool, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if len(ref) < MinRefLength {
		return Entry{}, false, nil
	}
	entries, err := s.LoadAll()
	if err != nil {
		return Entry{}, false, err
	}
	sortNewestFirst(entries)
	for _, entry := range entries {
		if strings.HasPrefix(entry.RawHash, ref) {
			return entry, true, nil
		}
	}
	return Entry{}, false, nil
}

// Latest returns the most recent entry.
func (s *Store) Latest() (Entry, bool, error) {
	entries, err := s.LoadAll()
	if err != nil {
		return Entry{}, false, err
	}
	if len(entries) == 0 {
		return Entry{}, false, nil
	}
	sortNewestFirst(entries)
	return entries[0], true, nil
}

func sortNewestFirst(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
}

// compact rewrites the index keeping only the newest entries that fit both
// the record cap and half the size cap (so the file has room to grow before
// the next compaction), then removes artifact files that no retained entry
// references. Best-effort and crash-safe: retained lines land in a temp file
// that is atomically renamed over the index.
func (s *Store) compact() {
	entries, err := s.LoadAll()
	if err != nil || len(entries) == 0 {
		return
	}
	sortNewestFirst(entries)
	retained := retainForCompaction(entries, compactRetainEntries, compactMaxIndexBytes/2)
	if len(retained) == len(entries) {
		return
	}
	if !s.rewriteIndex(retained) {
		return
	}
	s.removeOrphanedArtifacts(retained, entries[len(retained):])
}

// retainForCompaction walks newest-first and keeps entries until the record
// cap or byte budget is exceeded. The newest entry is always kept.
func retainForCompaction(entries []Entry, maxEntries int, maxBytes int) []Entry {
	total := 0
	for i, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return entries[:i]
		}
		total += len(line) + 1
		if i > 0 && (i >= maxEntries || total > maxBytes) {
			return entries[:i]
		}
	}
	return entries
}

func (s *Store) rewriteIndex(entries []Entry) bool {
	data, ok := encodeEntriesOldestFirst(entries)
	if !ok {
		return false
	}
	return replaceIndexFile(s.dir, s.path, data)
}

func encodeEntriesOldestFirst(entries []Entry) ([]byte, bool) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := len(entries) - 1; i >= 0; i-- {
		if err := enc.Encode(entries[i]); err != nil {
			return nil, false
		}
	}
	return buf.Bytes(), true
}

func replaceIndexFile(dir string, path string, data []byte) bool {
	tmp, err := os.CreateTemp(dir, "index-compact-*.tmp")
	if err != nil {
		return false
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return false
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return false
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return false
	}
	return true
}

// removeOrphanedArtifacts deletes the artifact files referenced only by
// dropped entries. Only files inside the dedup directory are touched.
func (s *Store) removeOrphanedArtifacts(retained []Entry, dropped []Entry) {
	live := make(map[string]struct{}, len(retained))
	for _, entry := range retained {
		live[entry.ArtifactPath] = struct{}{}
	}
	for _, entry := range dropped {
		if entry.ArtifactPath == "" {
			continue
		}
		if _, ok := live[entry.ArtifactPath]; ok {
			continue
		}
		if filepath.Dir(entry.ArtifactPath) != s.dir {
			continue
		}
		_ = os.Remove(entry.ArtifactPath)
	}
}
