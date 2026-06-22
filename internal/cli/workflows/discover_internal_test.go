package workflows

import (
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func TestLimitDiscoverOpportunitiesOrdersAndLimits(t *testing.T) {
	opportunities := []DiscoverOpportunity{
		{Recommendation: Recommendation{Command: "third", Priority: 60, Samples: 9}, CoverageScore: 10},
		{Recommendation: Recommendation{Command: "first", Priority: 90, Samples: 3}, CoverageScore: 20},
		{Recommendation: Recommendation{Command: "second", Priority: 90, Samples: 7}, CoverageScore: 20},
		{Recommendation: Recommendation{Command: "fourth", Priority: 90, Samples: 2}, CoverageScore: 5},
	}

	limited := limitDiscoverOpportunities(opportunities, 2)
	if len(limited) != 2 {
		t.Fatalf("unexpected limited opportunity count: %d", len(limited))
	}
	if limited[0].Command != "second" || limited[1].Command != "first" {
		t.Fatalf("unexpected opportunity order: %#v", limited)
	}

	all := limitDiscoverOpportunities(opportunities, 0)
	if len(all) != 4 {
		t.Fatalf("expected zero limit to keep all opportunities, got %d", len(all))
	}
	if all[2].Command != "fourth" || all[3].Command != "third" {
		t.Fatalf("unexpected full opportunity order: %#v", all)
	}
}

func TestBuildDiscoverOrdersAndLimitsOpportunities(t *testing.T) {
	records := []history.Record{
		{
			Timestamp:          time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			Command:            "terraform plan",
			CommandFingerprint: history.Fingerprint("terraform plan"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         20,
			ExitCode:           1,
			RawTokens:          200,
			FilteredTokens:     20,
			SavedTokens:        180,
			SavingsPct:         90,
			FallbackUsed:       true,
			TeePath:            "/tmp/terraform.log",
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
			Command:            "terraform plan",
			CommandFingerprint: history.Fingerprint("terraform plan"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         21,
			ExitCode:           1,
			RawTokens:          205,
			FilteredTokens:     21,
			SavedTokens:        184,
			SavingsPct:         89.7,
			FallbackUsed:       true,
			TeePath:            "/tmp/terraform-2.log",
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --stat",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --stat"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         18,
			RawTokens:          80,
			FilteredTokens:     80,
			SavedTokens:        0,
			SavingsPct:         0,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --stat",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --stat"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         19,
			RawTokens:          82,
			FilteredTokens:     82,
			SavedTokens:        0,
			SavingsPct:         0,
		},
	}

	report := BuildDiscover(records, 1)
	if report.Summary.Records != len(records) {
		t.Fatalf("unexpected discover record count: %#v", report.Summary)
	}
	if len(report.Opportunities) != 1 {
		t.Fatalf("expected discover limit to truncate results, got %#v", report.Opportunities)
	}
	if report.Opportunities[0].Command == "" || report.Opportunities[0].CoverageScore == 0 {
		t.Fatalf("expected discover opportunity to retain hotspot metadata, got %#v", report.Opportunities[0])
	}

	fullReport := BuildDiscover(records, 0)
	if len(fullReport.Opportunities) < 2 {
		t.Fatalf("expected zero discover limit to keep all opportunities, got %#v", fullReport.Opportunities)
	}
}
