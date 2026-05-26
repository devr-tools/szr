package bench_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/bench"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

func BenchmarkCompressionFixtures(b *testing.B) {
	fixtures, err := bench.Fixtures()
	if err != nil {
		b.Fatalf("fixtures: %v", err)
	}

	harness := bench.NewHarness(12)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, fixture := range fixtures {
			if _, err := harness.Measure(fixture); err != nil {
				b.Fatalf("measure %s: %v", fixture.Name, err)
			}
		}
	}
}

func BenchmarkFindSummaryStrategies(b *testing.B) {
	fixtures, err := bench.Fixtures()
	if err != nil {
		b.Fatalf("fixtures: %v", err)
	}
	var findFixture bench.Fixture
	found := false
	for _, fixture := range fixtures {
		if fixture.Name == "find-noisy-paths" {
			findFixture = fixture
			found = true
			break
		}
	}
	if !found {
		b.Fatal("missing find-noisy-paths fixture")
	}
	raw := findFixture.Execution.Stdout
	paths := filters.NonEmptyLines(raw)
	harness := bench.NewHarness(12)
	for _, candidate := range []struct {
		name string
		fn   func() string
	}{
		{
			name: "profile-inventory",
			fn: func() string {
				out, err := harness.Render(findFixture)
				if err != nil {
					b.Fatalf("render fixture: %v", err)
				}
				return out
			},
		},
		{
			name: "builtin-grouped",
			fn: func() string {
				return filters.SummarizeFindPathsGrouped(paths, 12)
			},
		},
	} {
		b.Run(candidate.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(raw)))
			sample := candidate.fn()
			reportFindTokenSavings(b, raw, sample)
			for i := 0; i < b.N; i++ {
				if got := candidate.fn(); strings.TrimSpace(got) == "" {
					b.Fatal("expected summarized find output")
				}
			}
		})
	}
}

func reportFindTokenSavings(b *testing.B, input string, output string) {
	inputTokens := history.EstimateTokens(input)
	outputTokens := history.EstimateTokens(output)
	if inputTokens <= 0 {
		return
	}
	retained := (float64(outputTokens) / float64(inputTokens)) * 100
	saved := 100 - retained
	b.ReportMetric(float64(inputTokens), "input_tokens")
	b.ReportMetric(float64(outputTokens), "output_tokens")
	b.ReportMetric(retained, "tokens_retained_pct")
	b.ReportMetric(saved, "tokens_saved_pct")
}
