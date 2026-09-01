package history_test

import (
	"math"
	"os"
	"path/filepath"
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
	assertProfileStat(t, statsByName["git-status"], func(stat history.ProfileStat) bool {
		return stat.SavedTokens == 80 && stat.DurationP50MS == 30 && stat.DurationP95MS == 30 && closeEnough(stat.AveragePct, 80, 0.01)
	})
	assertProfileStat(t, statsByName["go-test-json"], func(stat history.ProfileStat) bool {
		return stat.TeeCount == 1 && stat.Failures == 1 && closeEnough(stat.FailureRate, 100, 0.01) && stat.DurationP50MS == 90
	})
	assertProfileStat(t, statsByName["passthrough"], func(stat history.ProfileStat) bool {
		return stat.SavedTokens == 0 && stat.Failures == 1 && stat.DurationP50MS == 10
	})
	if len(summary.CommandHotspots) != 1 || summary.CommandHotspots[0].Command != "szr custom command" || !closeEnough(summary.CommandHotspots[0].FallbackRate, 100, 0.01) {
		t.Fatalf("unexpected command hotspots: %#v", summary.CommandHotspots)
	}
}

func assertProfileStat(t *testing.T, stat history.ProfileStat, ok func(history.ProfileStat) bool) {
	t.Helper()
	if !ok(stat) {
		t.Fatalf("unexpected profile stat: %#v", stat)
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

	invalidPath := string([]byte{'b', 'a', 'd', 0, '.', 'j', 's', 'o', 'n', 'l'})
	if _, err := history.New(invalidPath).LoadAll(); err == nil {
		t.Fatal("expected invalid path open error")
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
	if got := history.EstimateTokens("M  a"); got != 2 {
		t.Fatalf("expected tiny status output to estimate as 2, got %d", got)
	}
	if got := history.EstimateTokens("service/src/backend/api/routes/accounts.py"); got != 11 {
		t.Fatalf("expected path-like output to estimate as 11, got %d", got)
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

func TestStoreClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	if err := store.Append(history.Record{Command: "szr git status"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load after clear: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records after clear, got %#v", records)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cleared file: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected cleared file size 0, got %d", info.Size())
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
		// Only the first run fails: loosen suggestions are suppressed for
		// fingerprints that fail on (nearly) every run.
		exitCode := 0
		if i == 0 {
			exitCode = 1
		}
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 20, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr go build ./...",
			CommandFingerprint: history.Fingerprint("szr go build ./..."),
			Profile:            "go-build",
			ProfileConfidence:  "medium",
			DurationMS:         40,
			ExitCode:           exitCode,
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

func TestFindBudgetSuggestion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	fingerprint := history.Fingerprint("szr go test ./...")
	for i := 0; i < 4; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 24, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr go test ./...",
			CommandFingerprint: fingerprint,
			Profile:            "go-test-json",
			RawTokens:          120,
			FilteredTokens:     88,
			SavedTokens:        32,
			SavingsPct:         26.67,
			BytesEmitted:       360,
		}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := store.Append(history.Record{
		Timestamp:          time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC),
		Command:            "szr git status",
		CommandFingerprint: history.Fingerprint("szr git status"),
		Profile:            "git-status",
		RawTokens:          40,
		FilteredTokens:     8,
		SavedTokens:        32,
		SavingsPct:         80,
		BytesEmitted:       64,
	}); err != nil {
		t.Fatalf("append unrelated: %v", err)
	}

	suggestion, err := store.FindBudgetSuggestion(fingerprint, history.BudgetSuggestionOptions{})
	if err != nil {
		t.Fatalf("find suggestion: %v", err)
	}
	if suggestion == nil {
		t.Fatal("expected suggestion")
	}
	if suggestion.Fingerprint != fingerprint || suggestion.Direction != history.BudgetSuggestionTighten {
		t.Fatalf("unexpected suggestion: %#v", suggestion)
	}

	missing, err := store.FindBudgetSuggestion(history.Fingerprint("szr missing"), history.BudgetSuggestionOptions{})
	if err != nil {
		t.Fatalf("find missing suggestion: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing suggestion to be nil, got %#v", missing)
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
	if len(summary.CommandHotspots) != 1 || summary.CommandHotspots[0].FallbackRate != 100 {
		t.Fatalf("unexpected command hotspots: %#v", summary.CommandHotspots)
	}
	if len(summary.FingerprintHotspots) != 0 {
		t.Fatalf("expected singleton fingerprint hotspot to be suppressed, got %#v", summary.FingerprintHotspots)
	}
}

func TestSummaryPrioritizesMaterialHotspotsOverTinyNoise(t *testing.T) {
	records := []history.Record{
		{
			Command:        "szr git status",
			Profile:        "git-status",
			DurationMS:     18,
			RawTokens:      8,
			FilteredTokens: 12,
			SavedTokens:    -4,
			SavingsPct:     -50,
		},
		{
			Command:            "szr git diff --stat",
			CommandFingerprint: history.Fingerprint("szr git diff --stat"),
			Profile:            "git-diff",
			DurationMS:         34,
			RawTokens:          1200,
			FilteredTokens:     900,
			SavedTokens:        300,
			SavingsPct:         25,
		},
		{
			Command:            "szr git diff --stat",
			CommandFingerprint: history.Fingerprint("szr git diff --stat"),
			Profile:            "git-diff",
			DurationMS:         37,
			RawTokens:          1000,
			FilteredTokens:     720,
			SavedTokens:        280,
			SavingsPct:         28,
		},
		{
			Command:            "szr go test ./...",
			CommandFingerprint: history.Fingerprint("szr go test ./..."),
			Profile:            "go-test-json",
			DurationMS:         900,
			RawTokens:          5000,
			FilteredTokens:     300,
			SavedTokens:        4700,
			SavingsPct:         94,
		},
		{
			Command:            "szr go test ./...",
			CommandFingerprint: history.Fingerprint("szr go test ./..."),
			Profile:            "go-test-json",
			DurationMS:         880,
			RawTokens:          5200,
			FilteredTokens:     310,
			SavedTokens:        4890,
			SavingsPct:         94,
		},
	}

	summary := history.Summarize(records, 4)
	if len(summary.CommandHotspots) < 2 {
		t.Fatalf("expected multiple command hotspots, got %#v", summary.CommandHotspots)
	}
	if got := summary.CommandHotspots[0].Command; got != "szr git diff" {
		t.Fatalf("expected material poor-savings hotspot to rank first, got %#v", summary.CommandHotspots)
	}
	// A high-volume fingerprint that already compresses at 94% is a success:
	// it must appear in neither the hotspot table nor poor-savings, no
	// matter how many tokens it moves.
	for _, hotspot := range summary.CommandHotspots {
		if hotspot.Command == "szr go test" {
			t.Fatalf("expected healthy high-savings command out of hotspots, got %#v", summary.CommandHotspots)
		}
	}
	// The diff fingerprint averages ~26%: genuinely poor, so it belongs in
	// the poor-savings table; the healthy test fingerprint must not.
	for _, fingerprint := range summary.FingerprintHotspots {
		if fingerprint.Command == "szr go test ./..." {
			t.Fatalf("expected healthy fingerprint out of poor-savings table, got %#v", summary.FingerprintHotspots)
		}
	}
}

func closeEnough(got, want, tolerance float64) bool {
	return math.Abs(got-want) <= tolerance
}
