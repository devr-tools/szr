package history

import (
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	records := []Record{
		{
			Timestamp:      time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
			Command:        "szr git status",
			Profile:        "git-status",
			ExitCode:       0,
			RawTokens:      100,
			FilteredTokens: 20,
			SavedTokens:    80,
			SavingsPct:     80,
		},
		{
			Timestamp:      time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
			Command:        "szr go test ./...",
			Profile:        "go-test-json",
			ExitCode:       1,
			RawTokens:      120,
			FilteredTokens: 40,
			SavedTokens:    80,
			SavingsPct:     66.67,
		},
	}

	summary := Summarize(records, 8)
	if summary.Commands != 2 {
		t.Fatalf("expected 2 commands, got %d", summary.Commands)
	}
	if summary.Failures != 1 {
		t.Fatalf("expected 1 failure, got %d", summary.Failures)
	}
	if summary.SavedTokens != 160 {
		t.Fatalf("expected 160 saved tokens, got %d", summary.SavedTokens)
	}
	if len(summary.TopCommands) == 0 || summary.TopCommands[0].Command != "szr git status" {
		t.Fatalf("unexpected top commands: %#v", summary.TopCommands)
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens("abcd"); got != 1 {
		t.Fatalf("expected 1 token, got %d", got)
	}
	if got := EstimateTokens("abcdefgh"); got != 2 {
		t.Fatalf("expected 2 tokens, got %d", got)
	}
}
