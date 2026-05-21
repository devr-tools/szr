package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"szr/internal/history"
)

func TestHistoryStoreAndSummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	if store == nil {
		t.Fatal("expected store")
	}

	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
		Command:        "szr git status --short",
		Profile:        "git-status",
		ExitCode:       0,
		RawTokens:      100,
		FilteredTokens: 20,
		SavedTokens:    80,
		SavingsPct:     80,
	}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := os.WriteFile(path, append(mustReadFile(t, path), []byte("\nnot-json\n")...), 0o644); err != nil {
		t.Fatalf("inject bad line: %v", err)
	}
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Command:        "szr go test ./...",
		Profile:        "go-test-json",
		ExitCode:       1,
		RawTokens:      120,
		FilteredTokens: 40,
		SavedTokens:    80,
		SavingsPct:     66.67,
	}); err != nil {
		t.Fatalf("append 2: %v", err)
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	summary := history.Summarize(records, 1)
	if summary.Commands != 2 || summary.Failures != 1 || summary.SavedTokens != 160 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if len(summary.TopCommands) != 1 || summary.TopCommands[0].Command != "szr git status" {
		t.Fatalf("unexpected top commands: %#v", summary.TopCommands)
	}
	if len(summary.Recent) != 1 || summary.Recent[0].Command != "szr go test ./..." {
		t.Fatalf("unexpected recent: %#v", summary.Recent)
	}
	if summary.Profiles["git-status"] != 1 || summary.Profiles["go-test-json"] != 1 {
		t.Fatalf("unexpected profiles: %#v", summary.Profiles)
	}

	empty := history.Summarize(nil, 2)
	if empty.Commands != 0 || len(empty.Profiles) != 0 {
		t.Fatalf("unexpected empty summary: %#v", empty)
	}
}

func TestHistoryLoadAllErrorsAndHelpers(t *testing.T) {
	store := history.New(filepath.Join(t.TempDir(), "missing.jsonl"))
	records, err := store.LoadAll()
	if err != nil || len(records) != 0 {
		t.Fatalf("expected empty missing load, got records=%d err=%v", len(records), err)
	}

	emptyPath := filepath.Join(t.TempDir(), "empty.jsonl")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	records, err = history.New(emptyPath).LoadAll()
	if err != nil || len(records) != 0 {
		t.Fatalf("expected empty file load to succeed, got records=%d err=%v", len(records), err)
	}

	dirStore := history.New(t.TempDir())
	if _, err := dirStore.LoadAll(); err == nil {
		t.Fatal("expected directory load error")
	}

	protectedPath := filepath.Join(t.TempDir(), "protected.jsonl")
	if err := os.WriteFile(protectedPath, []byte("{}\n"), 0o000); err != nil {
		t.Fatalf("write protected file: %v", err)
	}
	if _, err := history.New(protectedPath).LoadAll(); err == nil {
		t.Fatal("expected protected file open error")
	}

	if got := history.EstimateTokens(""); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if got := history.EstimateTokens("abc"); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if got := history.EstimateTokens("abcdefgh"); got != 2 {
		t.Fatalf("expected 2, got %d", got)
	}

	summary := history.Summarize([]history.Record{
		{Command: "alpha beta gamma delta"},
		{Command: "alpha beta gamma epsilon"},
		{Command: ""},
	}, 5)
	if summary.TopCommands[0].Command != "alpha beta gamma" {
		t.Fatalf("unexpected normalized commands: %#v", summary.TopCommands)
	}

	appendStore := history.New(filepath.Join(t.TempDir(), "missing", "history.jsonl"))
	if err := appendStore.Append(history.Record{}); err == nil {
		t.Fatal("expected append error")
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return data
}

func TestHistoryLoadAllScannerError(t *testing.T) {
	longLine := strings.Repeat("x", 70*1024)
	path := filepath.Join(t.TempDir(), "long.jsonl")
	if err := os.WriteFile(path, []byte(longLine), 0o644); err != nil {
		t.Fatalf("write long file: %v", err)
	}

	store := history.New(path)
	if _, err := store.LoadAll(); err == nil {
		t.Fatal("expected scanner error for long line")
	}
}
