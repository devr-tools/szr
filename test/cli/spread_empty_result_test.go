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

func TestSpreadReportsEmptyResultsSeparatelyFromFallbacks(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{
			Timestamp:         time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
			Command:           "szr kubectl get pods",
			Profile:           "kubectl-get",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawTokens:         40,
			FilteredTokens:    40,
			FallbackUsed:      true,
			EmptyResult:       true,
		},
		{
			Timestamp:         time.Date(2026, 7, 1, 11, 0, 0, 0, time.UTC),
			Command:           "szr kubectl get pods",
			Profile:           "kubectl-get",
			ProfileConfidence: "high",
			DurationMS:        40,
			ExitCode:          0,
			RawTokens:         500,
			FilteredTokens:    50,
			SavedTokens:       450,
			SavingsPct:        90,
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
	if !strings.Contains(stdout, "empty results") || !strings.Contains(stdout, "(1/2)") {
		t.Fatalf("expected empty-results metric in spread summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "empty") || !strings.Contains(stdout, "fallback") {
		t.Fatalf("expected empty column beside fallback in profiles table, got %q", stdout)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal spread json: %v", err)
	}
	if payload.EmptyResults != 1 || payload.Fallbacks != 1 {
		t.Fatalf("expected split empty-result accounting in json payload, got %#v", payload)
	}
	if len(payload.ProfileStats) != 1 || payload.ProfileStats[0].EmptyResults != 1 {
		t.Fatalf("expected per-profile empty-result count, got %#v", payload.ProfileStats)
	}
}
