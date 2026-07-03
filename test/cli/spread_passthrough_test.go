package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSpreadReportsProxiedRunsSeparately(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{
			Timestamp:         time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
			Command:           "szr git status --short",
			Profile:           "git-status",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawTokens:         100,
			FilteredTokens:    20,
			SavedTokens:       80,
			SavingsPct:        80,
		},
		{
			Timestamp:         time.Date(2026, 6, 30, 11, 0, 0, 0, time.UTC),
			Command:           "szr proxy git diff",
			Profile:           "git-diff",
			ProfileConfidence: "medium",
			DurationMS:        40,
			ExitCode:          0,
			RawTokens:         900,
			FilteredTokens:    900,
			SavedTokens:       0,
			SavingsPct:        0,
			Passthrough:       true,
		},
		{
			Timestamp:         time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			Command:           "szr proxy git log",
			Profile:           "git-log",
			ProfileConfidence: "high",
			DurationMS:        20,
			ExitCode:          0,
			RawTokens:         300,
			FilteredTokens:    300,
			SavedTokens:       0,
			SavingsPct:        0,
			Passthrough:       true,
		},
	} {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "Total commands:  3") {
		t.Fatalf("expected proxied runs in total command count, got %q", stdout)
	}
	if !strings.Contains(stdout, "proxied (unfiltered): 2 commands, 1.2K tokens - excluded from savings analysis") {
		t.Fatalf("expected proxied summary line, got %q", stdout)
	}
	// Saved 80 of 100 filtered raw tokens; overall includes the 1200 proxied.
	if !strings.Contains(stdout, "(80.0% of filtered; 6.2% overall)") {
		t.Fatalf("expected filtered-vs-overall savings headline, got %q", stdout)
	}
	for _, unwanted := range []string{"szr proxy git", "git-diff", "git-log"} {
		if strings.Contains(stdout, unwanted) {
			t.Fatalf("expected proxied run %q to stay out of savings tables, got %q", unwanted, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal spread json: %v", err)
	}
	if payload.Commands != 3 || payload.PassthroughCommands != 2 || payload.PassthroughTokens != 1200 {
		t.Fatalf("unexpected json passthrough accounting: %#v", payload)
	}
	if payload.RawTokens != 1300 {
		t.Fatalf("expected proxied tokens in total tallies, got %#v", payload)
	}
	if payload.FilteredSavingsPct < 79.9 || payload.FilteredSavingsPct > 80.1 {
		t.Fatalf("expected filtered savings pct in json payload, got %#v", payload.FilteredSavingsPct)
	}
}
