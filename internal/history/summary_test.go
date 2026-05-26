package history

import (
	"testing"
	"time"
)

func TestCommandHotspotSeverityDownweightsTinyNegativeSavings(t *testing.T) {
	tiny := commandHotspotSeverity(CommandHotspot{
		Commands:      3,
		AveragePct:    -8,
		DurationP95MS: 20,
	}, 24, 28)

	material := commandHotspotSeverity(CommandHotspot{
		Commands:      3,
		AveragePct:    4,
		DurationP95MS: 20,
	}, 240, 230)

	if material <= tiny {
		t.Fatalf("expected material low-savings hotspot to outrank tiny overhead: material=%f tiny=%f", material, tiny)
	}
}

func TestCommandHotspotSeverityElevatesFallbackAndLatency(t *testing.T) {
	fallbackHeavy := commandHotspotSeverity(CommandHotspot{
		Commands:      5,
		AveragePct:    15,
		FallbackRate:  80,
		DurationP95MS: 180,
	}, 320, 220)

	mildLowSavings := commandHotspotSeverity(CommandHotspot{
		Commands:      5,
		AveragePct:    7,
		DurationP95MS: 30,
	}, 320, 220)

	if fallbackHeavy <= mildLowSavings {
		t.Fatalf("expected fallback-heavy latency hotspot to outrank mild low savings: fallback=%f low=%f", fallbackHeavy, mildLowSavings)
	}
}

func TestSummarizeCommandHotspotsOrdersTinyOverheadLast(t *testing.T) {
	now := time.Now()
	records := []Record{
		{
			Timestamp:      now.Add(-3 * time.Minute),
			Command:        "rg foo",
			Profile:        "search",
			DurationMS:     15,
			RawTokens:      20,
			FilteredTokens: 22,
			SavedTokens:    -2,
			SavingsPct:     -10,
		},
		{
			Timestamp:      now.Add(-2 * time.Minute),
			Command:        "rg foo",
			Profile:        "search",
			DurationMS:     18,
			RawTokens:      22,
			FilteredTokens: 23,
			SavedTokens:    -1,
			SavingsPct:     -4.5,
		},
		{
			Timestamp:      now.Add(-90 * time.Second),
			Command:        "find . -name '*.go'",
			Profile:        "search",
			DurationMS:     220,
			RawTokens:      320,
			FilteredTokens: 210,
			SavedTokens:    110,
			SavingsPct:     34.4,
			FallbackUsed:   true,
		},
		{
			Timestamp:      now.Add(-60 * time.Second),
			Command:        "find . -name '*.go'",
			Profile:        "search",
			DurationMS:     260,
			RawTokens:      300,
			FilteredTokens: 200,
			SavedTokens:    100,
			SavingsPct:     33.3,
			FallbackUsed:   true,
		},
		{
			Timestamp:      now.Add(-30 * time.Second),
			Command:        "git diff --name-only",
			Profile:        "directory-listing",
			DurationMS:     40,
			RawTokens:      260,
			FilteredTokens: 245,
			SavedTokens:    15,
			SavingsPct:     5.8,
		},
		{
			Timestamp:      now.Add(-10 * time.Second),
			Command:        "git diff --name-only",
			Profile:        "directory-listing",
			DurationMS:     45,
			RawTokens:      240,
			FilteredTokens: 225,
			SavedTokens:    15,
			SavingsPct:     6.2,
		},
	}

	summary := Summarize(records, 10)
	if len(summary.CommandHotspots) < 3 {
		t.Fatalf("expected hotspots for all command families, got %#v", summary.CommandHotspots)
	}

	if summary.CommandHotspots[0].Command != "find . -name" {
		t.Fatalf("expected fallback-heavy command first, got %#v", summary.CommandHotspots)
	}
	if summary.CommandHotspots[1].Command != "git diff --name-only" {
		t.Fatalf("expected material low-savings command second, got %#v", summary.CommandHotspots)
	}
	if summary.CommandHotspots[2].Command != "rg foo" {
		t.Fatalf("expected tiny-overhead command last, got %#v", summary.CommandHotspots)
	}
}
