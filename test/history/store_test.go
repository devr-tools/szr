package history_test

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestStoreLoadAll(t *testing.T) {
	_, records, _ := newHistorySummaryFixture(t)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestStoreSummaryTotals(t *testing.T) {
	_, _, summary := newHistorySummaryFixture(t)
	if summary.Commands != 3 || summary.Failures != 2 || summary.SavedTokens != 160 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.DurationP50MS != 30 || summary.DurationP95MS != 90 {
		t.Fatalf("unexpected duration percentiles: %#v", summary)
	}
	if !closeEnough(summary.FailureRate, 66.67, 0.01) || !closeEnough(summary.FallbackRate, 33.33, 0.01) || !closeEnough(summary.TeeRate, 33.33, 0.01) {
		t.Fatalf("unexpected rates: %#v", summary)
	}
}

func TestStoreSummaryCommandGroupings(t *testing.T) {
	_, _, summary := newHistorySummaryFixture(t)
	if len(summary.TopCommands) != 1 || summary.TopCommands[0].Command != "szr custom command" {
		t.Fatalf("unexpected top commands: %#v", summary.TopCommands)
	}
	if len(summary.Recent) != 1 || summary.Recent[0].Command != "szr custom command" {
		t.Fatalf("unexpected recent: %#v", summary.Recent)
	}
	if summary.Profiles["git-status"] != 1 || summary.Profiles["go-test-json"] != 1 || summary.Profiles["passthrough"] != 1 {
		t.Fatalf("unexpected profiles: %#v", summary.Profiles)
	}
}

func TestStoreSummaryProfileStats(t *testing.T) {
	_, _, summary := newHistorySummaryFixture(t)
	if len(summary.ProfileStats) != 3 {
		t.Fatalf("unexpected profile stats: %#v", summary.ProfileStats)
	}
	statsByName := map[string]history.ProfileStat{}
	for _, stat := range summary.ProfileStats {
		statsByName[stat.Name] = stat
	}
	if stat := statsByName["git-status"]; stat.SavedTokens != 80 || stat.DurationP50MS != 30 || stat.DurationP95MS != 30 || !closeEnough(stat.AveragePct, 80, 0.01) {
		t.Fatalf("unexpected git-status stat: %#v", stat)
	}
	if stat := statsByName["go-test-json"]; stat.TeeCount != 1 || stat.Failures != 1 || !closeEnough(stat.FailureRate, 100, 0.01) || stat.DurationP50MS != 90 {
		t.Fatalf("unexpected go-test-json stat: %#v", stat)
	}
	if stat := statsByName["passthrough"]; stat.SavedTokens != 0 || stat.Failures != 1 || stat.DurationP50MS != 10 {
		t.Fatalf("unexpected passthrough stat: %#v", stat)
	}
}

func TestStoreEmptySummary(t *testing.T) {
	empty := history.Summarize(nil, 2)
	if empty.Commands != 0 || len(empty.Profiles) != 0 || len(empty.ProfileStats) != 0 {
		t.Fatalf("unexpected empty summary: %#v", empty)
	}
}

func newHistorySummaryFixture(t *testing.T) (*history.Store, []history.Record, history.Summary) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	if store == nil {
		t.Fatal("expected store")
	}

	appendHistoryRecords(t, store, path)
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	return store, records, history.Summarize(records, 1)
}

func appendHistoryRecords(t *testing.T, store *history.Store, path string) {
	t.Helper()
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
		Command:        "szr git status --short",
		Profile:        "git-status",
		DurationMS:     30,
		ExitCode:       0,
		RawTokens:      100,
		FilteredTokens: 20,
		SavedTokens:    80,
		SavingsPct:     80,
	}); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := os.WriteFile(path, append(testutil.MustReadFile(t, path), []byte("\nnot-json\n")...), 0o644); err != nil {
		t.Fatalf("inject bad line: %v", err)
	}
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Command:        "szr go test ./...",
		Profile:        "go-test-json",
		DurationMS:     90,
		ExitCode:       1,
		RawTokens:      120,
		FilteredTokens: 40,
		SavedTokens:    80,
		SavingsPct:     66.67,
		TeePath:        "/tmp/go-test.log",
	}); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
		Command:        "szr custom command",
		Profile:        "passthrough",
		DurationMS:     10,
		ExitCode:       2,
		RawTokens:      60,
		FilteredTokens: 60,
		SavedTokens:    0,
		SavingsPct:     0,
	}); err != nil {
		t.Fatalf("append 3: %v", err)
	}
}

func TestLoadAllErrorsAndHelpers(t *testing.T) {
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
	if _, err := history.New(protectedPath).LoadAll(); err == nil && runtime.GOOS != "windows" {
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

func TestLoadAllScannerError(t *testing.T) {
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

func TestBudgetSuggestions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	for i := 0; i < 4; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 20, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr go build ./...",
			CommandFingerprint: history.Fingerprint("szr go build ./..."),
			Profile:            "go-build",
			ProfileConfidence:  "medium",
			DurationMS:         40,
			ExitCode:           1,
			RawTokens:          200,
			FilteredTokens:     12,
			SavedTokens:        188,
			SavingsPct:         94,
			BytesEmitted:       48,
			FallbackUsed:       i < 3,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	suggestions, err := store.SuggestBudgets(history.BudgetSuggestionOptions{Limit: 4})
	if err != nil {
		t.Fatalf("suggest budgets: %v", err)
	}
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %#v", suggestions)
	}
	suggestion := suggestions[0]
	if suggestion.Direction != history.BudgetSuggestionLoosen || suggestion.Reason != history.BudgetSuggestionFallbackHeavy {
		t.Fatalf("unexpected suggestion: %#v", suggestion)
	}
	if suggestion.Suggested.MaxLines <= 0 || suggestion.Samples != 4 || suggestion.Profile != "go-build" {
		t.Fatalf("unexpected suggestion payload: %#v", suggestion)
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all for summary: %v", err)
	}
	summary := history.Summarize(records, 8)
	if len(summary.BudgetSuggestions) != 1 || summary.BudgetSuggestions[0].Fingerprint != suggestion.Fingerprint {
		t.Fatalf("unexpected summary suggestions: %#v", summary.BudgetSuggestions)
	}
}

func TestSummaryHydratesLegacyFields(t *testing.T) {
	records := []history.Record{{
		Timestamp:      time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC),
		Command:        "szr custom command",
		Profile:        "passthrough",
		DurationMS:     12,
		ExitCode:       0,
		RawTokens:      20,
		FilteredTokens: 5,
		SavedTokens:    15,
		SavingsPct:     75,
	}}

	summary := history.Summarize(records, 4)
	if len(summary.Recent) != 1 {
		t.Fatalf("unexpected recent records: %#v", summary.Recent)
	}
	rec := summary.Recent[0]
	if rec.CommandFingerprint == "" || rec.ProfileConfidence != "low" || !rec.FallbackUsed {
		t.Fatalf("expected hydrated record fields, got %#v", rec)
	}
	if rec.RawBytesRead != 80 || rec.BytesParsed != 80 || rec.BytesEmitted != 20 {
		t.Fatalf("unexpected hydrated byte fields: %#v", rec)
	}
	if len(summary.ProfileStats) != 1 || summary.ProfileStats[0].Confidence != "low" || !closeEnough(summary.ProfileStats[0].FallbackRate, 100, 0.01) {
		t.Fatalf("unexpected hydrated profile stats: %#v", summary.ProfileStats)
	}
	if len(summary.FingerprintHotspots) != 1 || summary.FingerprintHotspots[0].Fingerprint == "" {
		t.Fatalf("unexpected fingerprint hotspots: %#v", summary.FingerprintHotspots)
	}
}

func closeEnough(got, want, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance
}
