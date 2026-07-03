package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

func TestPredictedTokenAllowance(t *testing.T) {
	t.Parallel()

	if got := filters.PredictedTokenAllowance("tiny output", 12); got != 0 {
		t.Fatalf("expected small raw output to disarm the self-cap, got %d", got)
	}

	large := strings.Repeat("some plain filler words here\n", 200)
	allowance := filters.PredictedTokenAllowance(large, 12)
	if allowance <= 0 {
		t.Fatalf("expected large raw output to arm the self-cap, got %d", allowance)
	}
	rawTokens := history.EstimateTokens(large)
	if allowance >= rawTokens/4 {
		t.Fatalf("expected allowance to stay near a fifth of raw (%d), got %d", rawTokens, allowance)
	}

	// The profile budget caps the allowance for huge raw output.
	huge := strings.Repeat("x y z w v u t s r q p o n m\n", 5000)
	if got := filters.PredictedTokenAllowance(huge, 12); got > 12*32 {
		t.Fatalf("expected allowance to respect the profile token budget, got %d", got)
	}
}

func TestFitPriorityLinesTierOrderAndBudgets(t *testing.T) {
	t.Parallel()

	candidates := []filters.PriorityLine{
		{Text: "failure: alpha failed hard", Tier: 0},
		{Text: "detail: something long and expensive that should lose to failures", Tier: 2},
		{Text: "failure: beta failed hard", Tier: 0},
		{Text: "summary: 2 failed", Tier: 1},
	}

	// Unlimited budgets keep everything in original order.
	all, omitted := filters.FitPriorityLines(candidates, 0, 0)
	if omitted != 0 || strings.Join(all, "\n") != "failure: alpha failed hard\ndetail: something long and expensive that should lose to failures\nfailure: beta failed hard\nsummary: 2 failed" {
		t.Fatalf("unexpected unlimited fit: omitted=%d lines=%q", omitted, all)
	}

	// A tight line budget grants tiers in order: failures, then the summary.
	tight, omitted := filters.FitPriorityLines(candidates, 3, 0)
	if omitted != 1 {
		t.Fatalf("expected one omitted line, got %d (%q)", omitted, tight)
	}
	for _, want := range []string{"failure: alpha failed hard", "failure: beta failed hard", "summary: 2 failed"} {
		if !strings.Contains(strings.Join(tight, "\n"), want) {
			t.Fatalf("expected %q to survive the line budget, got %q", want, tight)
		}
	}

	// Tier-0 lines bypass the token budget; lower tiers respect it.
	capped, _ := filters.FitPriorityLines(candidates, 0, filters.LineTokenCost("failure: alpha failed hard")+filters.LineTokenCost("failure: beta failed hard"))
	joined := strings.Join(capped, "\n")
	if !strings.Contains(joined, "failure: alpha failed hard") || !strings.Contains(joined, "failure: beta failed hard") {
		t.Fatalf("expected tier-0 lines to bypass the token budget, got %q", joined)
	}
	if strings.Contains(joined, "detail:") {
		t.Fatalf("expected the detail line to be dropped by the token budget, got %q", joined)
	}
}

func TestFitPriorityLinesWithMarkerRespectsBudget(t *testing.T) {
	t.Parallel()

	candidates := []filters.PriorityLine{
		{Text: "keep this line", Tier: 0},
		{Text: "drop this considerably more expensive line of text", Tier: 3},
	}
	budget := filters.LineTokenCost("keep this line") + filters.LineTokenCost("... +1 more lines")
	got := filters.FitPriorityLinesWithMarker(candidates, 0, budget)
	if !strings.Contains(got, "... +1 more lines") {
		t.Fatalf("expected omission marker within budget, got %q", got)
	}

	noRoom := filters.FitPriorityLinesWithMarker(candidates, 0, filters.LineTokenCost("keep this line"))
	if strings.Contains(noRoom, "more lines") {
		t.Fatalf("expected marker to be suppressed when over budget, got %q", noRoom)
	}
}
