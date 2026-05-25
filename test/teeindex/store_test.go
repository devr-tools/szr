package teeindex_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/teeindex"
)

func TestStoreListLatestFind(t *testing.T) {
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

	if err := store.Append(teeindex.Entry{
		Timestamp: time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
		Path:      firstPath,
		Command:   "szr go test ./...",
		Profile:   "go-test-json",
		ExitCode:  1,
	}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := store.Append(teeindex.Entry{
		Timestamp: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		Path:      secondPath,
		Command:   "szr cargo test",
		Profile:   "cargo-test",
		ExitCode:  101,
	}); err != nil {
		t.Fatalf("append second: %v", err)
	}

	entries, err := store.List(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 || entries[0].ID != "200_second" || entries[1].ID != "100_first" {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	latest, ok, err := store.Latest()
	if err != nil || !ok || latest.ID != "200_second" {
		t.Fatalf("unexpected latest entry=%#v ok=%t err=%v", latest, ok, err)
	}

	found, ok, err := store.Find("100_first")
	if err != nil || !ok || found.Path != firstPath {
		t.Fatalf("unexpected exact find entry=%#v ok=%t err=%v", found, ok, err)
	}

	found, ok, err = store.Find("200_")
	if err != nil || !ok || found.Path != secondPath {
		t.Fatalf("unexpected prefix find entry=%#v ok=%t err=%v", found, ok, err)
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
