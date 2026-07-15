package history_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func TestSummarizeSplitsEmptyResultsFromFallbacks(t *testing.T) {
	base := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	records := []history.Record{
		{
			Timestamp:      base,
			Command:        "kubectl get pods",
			Profile:        "kubectl-get",
			RawTokens:      40,
			FilteredTokens: 40,
			FallbackUsed:   true,
			EmptyResult:    true,
		},
		{
			Timestamp:      base.Add(time.Minute),
			Command:        "kubectl get pods",
			Profile:        "kubectl-get",
			RawTokens:      500,
			FilteredTokens: 480,
			SavedTokens:    20,
			SavingsPct:     4,
			FallbackUsed:   true,
		},
		{
			Timestamp:      base.Add(2 * time.Minute),
			Command:        "kubectl get pods",
			Profile:        "kubectl-get",
			RawTokens:      500,
			FilteredTokens: 30,
			SavedTokens:    470,
			SavingsPct:     94,
		},
	}

	summary := history.Summarize(records, 8)
	if summary.Fallbacks != 2 || summary.EmptyResults != 1 {
		t.Fatalf("expected fallbacks=2 emptyResults=1, got %#v", summary)
	}
	if summary.EmptyResultRate < 33.2 || summary.EmptyResultRate > 33.4 {
		t.Fatalf("unexpected empty-result rate: %#v", summary.EmptyResultRate)
	}

	if len(summary.ProfileStats) != 1 {
		t.Fatalf("expected one profile stat, got %#v", summary.ProfileStats)
	}
	stat := summary.ProfileStats[0]
	if stat.EmptyResults != 1 || stat.Fallbacks != 2 {
		t.Fatalf("expected profile stat to split empty results from fallbacks, got %#v", stat)
	}
	if stat.EmptyResultRate < 33.2 || stat.EmptyResultRate > 33.4 {
		t.Fatalf("unexpected profile empty-result rate: %#v", stat)
	}
}

// Empty-result runs must not feed the loosen/fallback_heavy budget trigger:
// a command that produced nothing renderable does not need a looser budget.
func TestSuggestBudgetsIgnoresEmptyResultFallbacks(t *testing.T) {
	emptyHeavy := emptyResultFallbackFixture(true)
	for _, suggestion := range history.SuggestBudgets(emptyHeavy, history.BudgetSuggestionOptions{Limit: 8}) {
		if suggestion.Command == "kubectl get pods" && suggestion.Reason == history.BudgetSuggestionFallbackHeavy {
			t.Fatalf("expected no fallback_heavy suggestion for empty-result runs, got %#v", suggestion)
		}
	}

	// The identical shape with genuine fallbacks must still trigger the
	// suggestion, proving the EmptyResult flag (not the record shape)
	// suppressed it.
	genuine := emptyResultFallbackFixture(false)
	suggestions := history.SuggestBudgets(genuine, history.BudgetSuggestionOptions{Limit: 8})
	for _, suggestion := range suggestions {
		if suggestion.Command == "kubectl get pods" && suggestion.Reason == history.BudgetSuggestionFallbackHeavy {
			return
		}
	}
	t.Fatalf("expected fallback_heavy suggestion for genuine fallbacks, got %#v", suggestions)
}

func emptyResultFallbackFixture(emptyResult bool) []history.Record {
	records := make([]history.Record, 0, 5)
	for i := 0; i < 5; i++ {
		records = append(records, history.Record{
			Timestamp:          time.Date(2026, 7, 1, 10+i, 0, 0, 0, time.UTC),
			Command:            "kubectl get pods",
			CommandFingerprint: history.Fingerprint("kubectl get pods"),
			Profile:            "kubectl-get",
			DurationMS:         60,
			ExitCode:           0,
			RawTokens:          80,
			FilteredTokens:     90,
			SavedTokens:        -10,
			SavingsPct:         -12.5,
			BytesEmitted:       360,
			FallbackUsed:       true,
			EmptyResult:        emptyResult,
		})
	}
	return records
}

// Records written before the empty_result field existed must decode
// tolerantly with the flag unset.
func TestLoadAllToleratesRecordsWithoutEmptyResultField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	line := `{"timestamp":"2026-06-30T10:00:00Z","command":"go build ./...","profile":"go-build","cwd":"/tmp","duration_ms":120,"exit_code":1,"raw_bytes":400,"filtered_bytes":120,"raw_tokens":100,"filtered_tokens":30,"saved_tokens":70,"savings_pct":70,"fallback_used":true}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatalf("write legacy history: %v", err)
	}

	records, err := history.New(path).LoadAll()
	if err != nil {
		t.Fatalf("load legacy history: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one legacy record, got %#v", records)
	}
	if records[0].EmptyResult {
		t.Fatalf("expected legacy record to leave EmptyResult unset, got %#v", records[0])
	}
	if !records[0].FallbackUsed || records[0].RawTokens != 100 {
		t.Fatalf("unexpected legacy record decode: %#v", records[0])
	}
}
