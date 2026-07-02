package history_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPassthroughRecordRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.New(path)
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		Command:        "szr proxy git status",
		Profile:        "passthrough",
		RawTokens:      500,
		FilteredTokens: 500,
		Passthrough:    true,
	}); err != nil {
		t.Fatalf("append passthrough: %v", err)
	}
	if err := store.Append(history.Record{
		Timestamp:      time.Date(2026, 6, 30, 11, 0, 0, 0, time.UTC),
		Command:        "szr git status",
		Profile:        "git-status",
		RawTokens:      100,
		FilteredTokens: 20,
	}); err != nil {
		t.Fatalf("append filtered: %v", err)
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) != 2 || !records[0].Passthrough || records[1].Passthrough {
		t.Fatalf("unexpected passthrough round trip: %#v", records)
	}

	raw := string(testutil.MustReadFile(t, path))
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected history lines: %q", raw)
	}
	if !strings.Contains(lines[0], `"passthrough":true`) {
		t.Fatalf("expected passthrough flag in serialized record, got %q", lines[0])
	}
	if strings.Contains(lines[1], `"passthrough"`) {
		t.Fatalf("expected omitted passthrough key on filtered record, got %q", lines[1])
	}
}

func TestSummarizeExcludesPassthroughFromSavingsAnalysis(t *testing.T) {
	records := passthroughSummaryFixture()
	summary := history.Summarize(records, 8)

	if summary.Commands != 6 || summary.PassthroughCommands != 4 || summary.PassthroughTokens != 4000 {
		t.Fatalf("unexpected totals: %#v", summary)
	}
	if summary.RawTokens != 4200 || summary.FilteredTokens != 4040 {
		t.Fatalf("expected passthrough runs to stay in token tallies, got %#v", summary)
	}
	if !closeEnough(summary.AveragePct, 80, 0.01) {
		t.Fatalf("expected average savings over filtered runs only, got %#v", summary.AveragePct)
	}

	assertNoPassthroughSavingsArtifacts(t, summary)
}

func assertNoPassthroughSavingsArtifacts(t *testing.T, summary history.Summary) {
	t.Helper()
	for _, cmd := range summary.TopCommands {
		if strings.Contains(cmd.Command, "proxy") {
			t.Fatalf("expected proxied command excluded from top commands, got %#v", summary.TopCommands)
		}
	}
	for _, stat := range summary.ProfileStats {
		if stat.Name == "git-diff" {
			t.Fatalf("expected proxied-only profile excluded from profile stats, got %#v", summary.ProfileStats)
		}
	}
	for _, hotspot := range summary.CommandHotspots {
		if strings.Contains(hotspot.Command, "proxy") {
			t.Fatalf("expected proxied command excluded from hotspots, got %#v", summary.CommandHotspots)
		}
	}
	for _, fingerprint := range summary.FingerprintHotspots {
		if strings.Contains(fingerprint.Command, "proxy") {
			t.Fatalf("expected proxied fingerprint excluded, got %#v", summary.FingerprintHotspots)
		}
	}
	for _, suggestion := range summary.BudgetSuggestions {
		if strings.Contains(suggestion.Command, "proxy") {
			t.Fatalf("expected proxied command excluded from budget suggestions, got %#v", summary.BudgetSuggestions)
		}
	}
}

func TestSummarizeKeepsLegacyPassthroughProfileRecords(t *testing.T) {
	// Old records have no passthrough flag; profile=="passthrough" alone must
	// not trigger exclusion because that profile also serves filtered
	// fallback runs.
	records := []history.Record{{
		Timestamp:      time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		Command:        "szr custom command",
		Profile:        "passthrough",
		RawTokens:      120,
		FilteredTokens: 90,
		SavedTokens:    30,
		SavingsPct:     25,
	}}
	summary := history.Summarize(records, 4)
	if summary.PassthroughCommands != 0 || summary.PassthroughTokens != 0 {
		t.Fatalf("expected legacy records to stay untagged, got %#v", summary)
	}
	if len(summary.TopCommands) != 1 || !closeEnough(summary.AveragePct, 25, 0.01) {
		t.Fatalf("expected legacy record in savings analysis, got %#v", summary)
	}
}

func passthroughSummaryFixture() []history.Record {
	records := make([]history.Record, 0, 6)
	for i := 0; i < 4; i++ {
		records = append(records, history.Record{
			Timestamp:          time.Date(2026, 6, 30, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr proxy git diff",
			CommandFingerprint: history.Fingerprint("szr proxy git diff"),
			Profile:            "git-diff",
			DurationMS:         40,
			RawTokens:          1000,
			FilteredTokens:     1000,
			SavedTokens:        0,
			SavingsPct:         0,
			BytesEmitted:       4000,
			Passthrough:        true,
		})
	}
	for i := 0; i < 2; i++ {
		records = append(records, history.Record{
			Timestamp:          time.Date(2026, 6, 30, 15+i, 0, 0, 0, time.UTC),
			Command:            "szr git status",
			CommandFingerprint: history.Fingerprint("szr git status"),
			Profile:            "git-status",
			DurationMS:         20,
			RawTokens:          100,
			FilteredTokens:     20,
			SavedTokens:        80,
			SavingsPct:         80,
			BytesEmitted:       80,
		})
	}
	return records
}

func TestSuggestBudgetsSkipsPassthroughRecords(t *testing.T) {
	records := passthroughSummaryFixture()
	suggestions := history.SuggestBudgets(records, history.BudgetSuggestionOptions{Limit: 8})
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion.Command, "proxy") {
			t.Fatalf("expected no suggestion for proxied runs, got %#v", suggestions)
		}
	}

	// The same shape without the flag must produce a tighten suggestion,
	// proving the exclusion (not sample thresholds) suppressed it.
	unflagged := passthroughSummaryFixture()
	for i := range unflagged {
		unflagged[i].Passthrough = false
	}
	suggestions = history.SuggestBudgets(unflagged, history.BudgetSuggestionOptions{Limit: 8})
	found := false
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion.Command, "proxy") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unflagged fixture to trigger a suggestion, got %#v", suggestions)
	}
}
