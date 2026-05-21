package bench_test

import (
	"testing"

	"szr/internal/bench"
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
