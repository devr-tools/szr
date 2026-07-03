package history_test

import (
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

// Regression: the "poor savings fingerprints" table used a volume-weighted
// severity that surfaced high-volume 99%-savings fingerprints as "poor".
// Only genuinely poor rows (average savings below the threshold, or
// negative) may appear, ranked by residual (post-filter) token volume.
func TestFingerprintHotspotsListOnlyGenuinelyPoorRows(t *testing.T) {
	records := poorSavingsFixture()
	summary := history.Summarize(records, 8)

	for _, stat := range summary.FingerprintHotspots {
		if stat.Command == "kubectl get pods -A -o json" {
			t.Fatalf("expected high-savings high-volume fingerprint to stay out of poor-savings table, got %#v", summary.FingerprintHotspots)
		}
		if stat.AveragePct >= 40 {
			t.Fatalf("expected only rows below the poor-savings threshold, got %#v", stat)
		}
	}

	if len(summary.FingerprintHotspots) != 3 {
		t.Fatalf("expected the three poor fingerprints, got %#v", summary.FingerprintHotspots)
	}
	// Ranked by residual token volume, biggest poor performer first.
	if summary.FingerprintHotspots[0].Command != "go vet ./..." {
		t.Fatalf("expected big 20%%-savings fingerprint first, got %#v", summary.FingerprintHotspots)
	}
	if summary.FingerprintHotspots[1].Command != "cat notes.txt" {
		t.Fatalf("expected small 20%%-savings fingerprint second, got %#v", summary.FingerprintHotspots)
	}
	if summary.FingerprintHotspots[2].Command != "echo tiny" {
		t.Fatalf("expected negative-savings fingerprint included and ranked by residual volume, got %#v", summary.FingerprintHotspots)
	}
}

func poorSavingsFixture() []history.Record {
	records := make([]history.Record, 0, 13)
	base := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	// Dominant high-savings traffic: must never be reported as poor.
	for i := 0; i < 5; i++ {
		records = append(records, history.Record{
			Timestamp:          base.Add(time.Duration(i) * time.Minute),
			Command:            "kubectl get pods -A -o json",
			CommandFingerprint: history.Fingerprint("kubectl get pods -A -o json"),
			Profile:            "kubectl-json",
			DurationMS:         120,
			RawTokens:          2000,
			FilteredTokens:     20,
			SavedTokens:        1980,
			SavingsPct:         99,
		})
	}
	// Genuinely poor, big residual volume: must rank first.
	for i := 0; i < 3; i++ {
		records = append(records, history.Record{
			Timestamp:          base.Add(time.Duration(10+i) * time.Minute),
			Command:            "go vet ./...",
			CommandFingerprint: history.Fingerprint("go vet ./..."),
			Profile:            "generic-summary",
			DurationMS:         300,
			RawTokens:          1500,
			FilteredTokens:     1200,
			SavedTokens:        300,
			SavingsPct:         20,
		})
	}
	// Genuinely poor, small residual volume: must rank after the big one.
	for i := 0; i < 3; i++ {
		records = append(records, history.Record{
			Timestamp:          base.Add(time.Duration(20+i) * time.Minute),
			Command:            "cat notes.txt",
			CommandFingerprint: history.Fingerprint("cat notes.txt"),
			Profile:            "generic-summary",
			DurationMS:         12,
			RawTokens:          150,
			FilteredTokens:     120,
			SavedTokens:        30,
			SavingsPct:         20,
		})
	}
	// Negative savings (output grew): always qualifies as poor.
	for i := 0; i < 2; i++ {
		records = append(records, history.Record{
			Timestamp:          base.Add(time.Duration(30+i) * time.Minute),
			Command:            "echo tiny",
			CommandFingerprint: history.Fingerprint("echo tiny"),
			Profile:            "generic-summary",
			DurationMS:         5,
			RawTokens:          30,
			FilteredTokens:     45,
			SavedTokens:        -15,
			SavingsPct:         -50,
		})
	}
	return records
}

// Regression: fallback-heavy fingerprints whose runs (almost) all exit
// nonzero used to get loosen/fallback_heavy budget suggestions. A command
// that fails by design (kubectl without a cluster, grep with no matches)
// needs investigation, not a looser budget.
func TestSuggestBudgetsSkipsLoosenForAlwaysFailingFingerprints(t *testing.T) {
	failing := alwaysFailingFallbackFixture(5)
	suggestions := history.SuggestBudgets(failing, history.BudgetSuggestionOptions{Limit: 8})
	for _, suggestion := range suggestions {
		if suggestion.Command == "kubectl get pods" {
			t.Fatalf("expected no suggestion for 100%%-failure fingerprint, got %#v", suggestion)
		}
	}

	// The same shape with successful exits must produce the loosen
	// suggestion, proving the failure-rate gate (not sample or fallback
	// thresholds) suppressed it.
	healthy := alwaysFailingFallbackFixture(0)
	suggestions = history.SuggestBudgets(healthy, history.BudgetSuggestionOptions{Limit: 8})
	assertLoosenFallbackHeavy(t, suggestions)

	// At exactly 80% (4 of 5 failing) the gate does not yet trip.
	borderline := alwaysFailingFallbackFixture(4)
	suggestions = history.SuggestBudgets(borderline, history.BudgetSuggestionOptions{Limit: 8})
	assertLoosenFallbackHeavy(t, suggestions)
}

func assertLoosenFallbackHeavy(t *testing.T, suggestions []history.BudgetSuggestion) {
	t.Helper()
	for _, suggestion := range suggestions {
		if suggestion.Command != "kubectl get pods" {
			continue
		}
		if suggestion.Direction != history.BudgetSuggestionLoosen || suggestion.Reason != history.BudgetSuggestionFallbackHeavy {
			t.Fatalf("unexpected suggestion shape: %#v", suggestion)
		}
		return
	}
	t.Fatalf("expected loosen/fallback_heavy suggestion, got %#v", suggestions)
}

func alwaysFailingFallbackFixture(failures int) []history.Record {
	records := make([]history.Record, 0, 5)
	for i := 0; i < 5; i++ {
		exitCode := 0
		if i < failures {
			exitCode = 1
		}
		records = append(records, history.Record{
			Timestamp:          time.Date(2026, 6, 30, 10+i, 0, 0, 0, time.UTC),
			Command:            "kubectl get pods",
			CommandFingerprint: history.Fingerprint("kubectl get pods"),
			Profile:            "passthrough",
			DurationMS:         60,
			ExitCode:           exitCode,
			RawTokens:          80,
			FilteredTokens:     90,
			SavedTokens:        -10,
			SavingsPct:         -12.5,
			BytesEmitted:       360,
			FallbackUsed:       true,
		})
	}
	return records
}
