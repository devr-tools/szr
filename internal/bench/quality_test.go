package bench

import "testing"

func TestScoreQualityTreatsTinyNegativeSavingsAsOverhead(t *testing.T) {
	score, issues := scoreQuality(1, nil, 0, Measurement{
		RawTokens:       12,
		FilteredTokens:  15,
		SavedTokens:     -3,
		TokenSavingsPct: -25,
	})

	if score != 90 {
		t.Fatalf("expected softened tiny-output penalty, got score=%d issues=%v", score, issues)
	}
	if len(issues) != 1 || issues[0] != "tiny_output_overhead" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}

func TestScoreQualityPenalizesMaterialNegativeSavings(t *testing.T) {
	score, issues := scoreQuality(2, nil, 0, Measurement{
		RawTokens:       120,
		FilteredTokens:  160,
		SavedTokens:     -40,
		TokenSavingsPct: -33.3,
	})

	if score != 45 {
		t.Fatalf("expected material negative savings penalty, got score=%d issues=%v", score, issues)
	}
	if len(issues) != 2 || issues[0] != "low_token_savings" || issues[1] != "negative_token_savings" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}

func TestScoreQualityIgnoresLowSavingsForTinyOutputs(t *testing.T) {
	score, issues := scoreQuality(1, nil, 0, Measurement{
		RawTokens:       20,
		FilteredTokens:  19,
		SavedTokens:     1,
		TokenSavingsPct: 5,
	})

	if score != 100 {
		t.Fatalf("expected tiny output not to be penalized, got score=%d issues=%v", score, issues)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}
