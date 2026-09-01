package teeindex_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/teeindex"
)

func TestStoreListOrder(t *testing.T) {
	store, _, _ := newSeededTeeStore(t)
	entries, err := store.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 || entries[0].ID != "200_second" || entries[1].ID != "100_first" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestStoreLatest(t *testing.T) {
	store, _, _ := newSeededTeeStore(t)
	latest, ok, err := store.Latest()
	if err != nil || !ok || latest.ID != "200_second" {
		t.Fatalf("unexpected latest entry=%#v ok=%t err=%v", latest, ok, err)
	}
}

func TestStoreFind(t *testing.T) {
	store, firstPath, secondPath := newSeededTeeStore(t)

	found, ok, err := store.Find("100_first")
	if err != nil || !ok || found.Path != firstPath {
		t.Fatalf("unexpected exact find entry=%#v ok=%t err=%v", found, ok, err)
	}

	found, ok, err = store.Find("200_")
	if err != nil || !ok || found.Path != secondPath {
		t.Fatalf("unexpected prefix find entry=%#v ok=%t err=%v", found, ok, err)
	}
}

func TestStoreRead(t *testing.T) {
	store, _, _ := newSeededTeeStore(t)
	found, ok, err := store.Find("200_")
	if err != nil || !ok {
		t.Fatalf("find entry for read: ok=%t err=%v", ok, err)
	}
	data, err := store.Read(found)
	if err != nil || string(data) != "second\n" {
		t.Fatalf("unexpected read data=%q err=%v", string(data), err)
	}
}

func TestStoreSearchAndReplace(t *testing.T) {
	dir := t.TempDir()
	store := teeindex.New(dir)

	firstPath := filepath.Join(dir, "100_first.log")
	secondPath := filepath.Join(dir, "200_second.log")
	if err := os.WriteFile(firstPath, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first tee: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second tee: %v", err)
	}

	entries := []teeindex.Entry{
		{
			Timestamp: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			Path:      firstPath,
			Command:   "terraform plan",
			Profile:   "passthrough",
			ExitCode:  1,
		},
		{
			Timestamp: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
			Path:      secondPath,
			Command:   "cargo test",
			Profile:   "cargo-test",
			ExitCode:  101,
		},
	}
	if err := store.Replace(entries); err != nil {
		t.Fatalf("replace entries: %v", err)
	}

	matches, err := store.Search("cargo", 10)
	if err != nil {
		t.Fatalf("search entries: %v", err)
	}
	if len(matches) != 1 || matches[0].Command != "cargo test" {
		t.Fatalf("unexpected tee search matches: %#v", matches)
	}

	if err := store.Replace(entries[:1]); err != nil {
		t.Fatalf("replace subset: %v", err)
	}
	list, err := store.List(10)
	if err != nil {
		t.Fatalf("list replaced entries: %v", err)
	}
	if len(list) != 1 || list[0].Command != "terraform plan" {
		t.Fatalf("unexpected replaced tee entries: %#v", list)
	}
}

func newSeededTeeStore(t *testing.T) (*teeindex.Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	store := teeindex.New(dir)

	firstPath := filepath.Join(dir, "100_first.log")
	secondPath := filepath.Join(dir, "200_second.log")
	if err := os.WriteFile(firstPath, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first tee: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second tee: %v", err)
	}

	for _, entry := range []teeindex.Entry{
		{
			Timestamp: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			Path:      firstPath,
			Command:   "szr go test ./...",
			Profile:   "go-test-json",
			ExitCode:  1,
		},
		{
			Timestamp: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
			Path:      secondPath,
			Command:   "szr cargo test",
			Profile:   "cargo-test",
			ExitCode:  101,
		},
	} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append tee entry: %v", err)
		}
	}

	return store, firstPath, secondPath
}

func TestLoadAllSkipsOversizedLinesAndClipsCommands(t *testing.T) {
	dir := t.TempDir()
	store := teeindex.New(dir)
	logPath := filepath.Join(dir, "100_first.log")
	if err := os.WriteFile(logPath, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write tee log: %v", err)
	}

	command := "szr rg needle " + strings.Repeat("path/to/file ", 100_000)
	if err := store.Append(teeindex.Entry{
		Timestamp: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		Path:      logPath,
		Command:   command,
		Profile:   "rg",
	}); err != nil {
		t.Fatalf("append oversized command: %v", err)
	}
	indexPath := filepath.Join(dir, "index.jsonl")
	file, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	if _, err := file.WriteString(strings.Repeat("x", 2<<20) + "\n"); err != nil {
		t.Fatalf("write oversized line: %v", err)
	}
	_ = file.Close()

	entries, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected the clipped entry to survive, got %#v", entries)
	}
	if len(entries[0].Command) >= len(command) || !strings.HasPrefix(entries[0].Command, "szr rg needle ") {
		t.Fatalf("expected the command clipped to its leading tokens, got %d bytes", len(entries[0].Command))
	}
}
