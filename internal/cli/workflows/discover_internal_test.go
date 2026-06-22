package workflows

import "testing"

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
