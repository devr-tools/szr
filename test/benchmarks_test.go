package test

import (
	"testing"

	"szr/internal/bench"
)

func BenchmarkCompressionFixtures(b *testing.B) {
	fixtures := bench.MustFixtures()
	harness := bench.NewHarness(12)

	for _, fixture := range fixtures {
		fixture := fixture
		baseline, err := harness.Measure(fixture)
		if err != nil {
			b.Fatalf("baseline measure %s: %v", fixture.Name, err)
		}

		b.Run(fixture.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(baseline.RawBytes))
			b.ReportMetric(baseline.ByteRatio, "byte_ratio")
			b.ReportMetric(baseline.TokenRatio, "token_ratio")
			b.ReportMetric(baseline.ByteSavingsPct, "bytes_saved_%")
			b.ReportMetric(baseline.TokenSavingsPct, "tokens_saved_%")
			b.ReportMetric(float64(baseline.FilteredTokens), "tokens_out")
			b.ReportMetric(float64(baseline.RawTokens), "tokens_in")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := harness.Measure(fixture); err != nil {
					b.Fatalf("measure %s: %v", fixture.Name, err)
				}
			}
		})
	}
}
