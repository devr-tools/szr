package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// spreadArchivedApp seeds two on-disk records plus a totals sidecar standing in
// for runs a previous compaction removed.
func spreadArchivedApp(t *testing.T) (*cli.App, config.Paths) {
	t.Helper()
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{Command: "git status --short", Profile: "git-status", RawTokens: 100, FilteredTokens: 40, SavedTokens: 60, SavingsPct: 60, DurationMS: 10},
		{Command: "go test ./...", Profile: "go-test", RawTokens: 900, FilteredTokens: 60, SavedTokens: 840, SavingsPct: 93, DurationMS: 30},
	} {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	totals := history.Totals{
		Version:         history.TotalsVersion,
		Commands:        1_000,
		RawTokens:       1_000_000,
		FilteredTokens:  100_000,
		SavedTokens:     900_000,
		TotalDurationMS: 60_000,
		Failures:        50,
		SavingsPctSum:   90_000,
		DroppedRecords:  2,
	}
	data, err := json.Marshal(totals)
	if err != nil {
		t.Fatalf("marshal totals: %v", err)
	}
	sidecar := filepath.Join(filepath.Dir(paths.HistoryFile), "history-totals.json")
	if err := os.WriteFile(sidecar, data, 0o644); err != nil {
		t.Fatalf("seed totals sidecar: %v", err)
	}

	return cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths)), paths
}

func TestSpreadReportsArchivedTotalsAndScope(t *testing.T) {
	app, _ := spreadArchivedApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || stderr != "" {
		t.Fatalf("spread failed code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "Total commands:  1002") {
		t.Fatalf("expected lifetime command count, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "archived history") || !strings.Contains(stdout, "1000 older runs") {
		t.Fatalf("expected the archived-scope note, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 runs still on disk") {
		t.Fatalf("expected the note to scope the tables to on-disk runs, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 unreadable records were discarded") {
		t.Fatalf("expected the note to admit dropped records, got:\n%s", stdout)
	}
	// The tables still describe only the records on disk.
	if !strings.Contains(stdout, "git status --short") || !strings.Contains(stdout, "go test ./...") {
		t.Fatalf("expected the window tables to render, got:\n%s", stdout)
	}
}

func TestSpreadJSONExposesArchivedCounts(t *testing.T) {
	app, _ := spreadArchivedApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("spread --json failed code=%d stderr=%q", code, stderr)
	}
	var payload struct {
		Commands               int `json:"commands"`
		SavedTokens            int `json:"saved_tokens"`
		ArchivedCommands       int `json:"archived_commands"`
		ArchivedDroppedRecords int `json:"archived_dropped_records"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode spread json: %v", err)
	}
	if payload.Commands != 1_002 || payload.SavedTokens != 900_900 {
		t.Fatalf("expected combined totals in json, got %#v", payload)
	}
	if payload.ArchivedCommands != 1_000 || payload.ArchivedDroppedRecords != 2 {
		t.Fatalf("expected archived counts in json, got %#v", payload)
	}
}

func TestSpreadCostPricesArchivedTokens(t *testing.T) {
	app, _ := spreadArchivedApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--cost", "--rate", "3")
	if code != 0 || stderr != "" {
		t.Fatalf("spread --cost failed code=%d stderr=%q", code, stderr)
	}
	// 900,900 saved input tokens at $3/Mtok is about $2.70; a window-only
	// report would have shown less than a cent.
	if !strings.Contains(stdout, "2.7") {
		t.Fatalf("expected lifetime savings to be priced, got:\n%s", stdout)
	}
}

func TestClearSpreadDiscardsArchivedTotals(t *testing.T) {
	app, paths := spreadArchivedApp(t)

	code, _, stderr := testutil.RunApp(t, app, "clear-spread")
	if code != 0 || stderr != "" {
		t.Fatalf("clear-spread failed code=%d stderr=%q", code, stderr)
	}
	sidecar := filepath.Join(filepath.Dir(paths.HistoryFile), "history-totals.json")
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("expected the totals sidecar to be removed, err=%v", err)
	}

	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || stderr != "" {
		t.Fatalf("spread after clear failed code=%d stderr=%q", code, stderr)
	}
	if strings.TrimSpace(stdout) != "no tracked commands yet" {
		t.Fatalf("expected a cleared spread report, got:\n%s", stdout)
	}
}
