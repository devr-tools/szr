package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/history"
	"szr/internal/teeindex"
	"szr/test/testutil"
)

func TestTeeListAndRead(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := teeindex.New(paths.TeeDir)
	firstPath := filepath.Join(paths.TeeDir, "100_first.log")
	secondPath := filepath.Join(paths.TeeDir, "200_second.log")
	testutil.MustWriteFile(t, firstPath, "first artifact\n")
	testutil.MustWriteFile(t, secondPath, "second artifact\n")

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

	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths))

	code, stdout, stderr := testutil.RunApp(t, app, "tee")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected tee list stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"tee artifacts:", "200_second", "cargo-test", "szr cargo test"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected tee list output %q in %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "--latest")
	if code != 0 || stderr != "" || stdout != "second artifact\n" {
		t.Fatalf("unexpected tee latest stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "100_first")
	if code != 0 || stderr != "" || stdout != "first artifact\n" {
		t.Fatalf("unexpected tee id stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected tee json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var entries []teeindex.Entry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("decode tee json: %v", err)
	}
	if len(entries) != 2 || entries[0].ID != "200_second" {
		t.Fatalf("unexpected tee json entries: %#v", entries)
	}
}

func TestTeeErrors(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths))

	code, stdout, stderr := testutil.RunApp(t, app, "tee")
	if code != 0 || strings.TrimSpace(stdout) != "no tee artifacts yet" || stderr != "" {
		t.Fatalf("unexpected empty tee stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "--latest")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "no tee artifacts found") {
		t.Fatalf("unexpected latest-missing stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "--bad")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown tee flag") {
		t.Fatalf("unexpected bad-flag stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "tee", "one", "two")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "at most one artifact id") {
		t.Fatalf("unexpected multi-id stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	store := teeindex.New(paths.TeeDir)
	missingPath := filepath.Join(paths.TeeDir, "300_missing.log")
	if err := store.Append(teeindex.Entry{
		Timestamp: time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Path:      missingPath,
		Command:   "szr npm test",
		Profile:   "js-package-test",
		ExitCode:  1,
	}); err != nil {
		t.Fatalf("append missing tee entry: %v", err)
	}
	code, stdout, stderr = testutil.RunApp(t, app, "tee", "300_missing")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "tee artifact unavailable") {
		t.Fatalf("unexpected missing artifact stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}
