package bench_test

import (
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/bench"
	"github.com/devr-tools/szr/internal/engine"
)

func TestBenchmarkAggregatesAndSignals(t *testing.T) {
	harness := bench.NewHarnessWithProfiles([]engine.Profile{{
		Name: "fallback",
		Render: func(engine.Invocation, engine.Execution) string {
			return ""
		},
	}})

	fixture := bench.Fixture{
		Name:            "fallback-heavy",
		Class:           "edge",
		ProfileName:     "fallback",
		MinQualityScore: 0,
		Execution: engine.Execution{
			Stdout: strings.Repeat("line that should stay verbose\n", 40),
		},
	}

	measurement, err := harness.Benchmark(fixture, 5)
	if err != nil {
		t.Fatalf("benchmark: %v", err)
	}
	if measurement.Durations.Samples != 5 {
		t.Fatalf("unexpected samples: %#v", measurement.Durations)
	}
	if measurement.FallbackRate != 100 {
		t.Fatalf("expected full fallback rate, got %#v", measurement)
	}
	if measurement.Quality.Score >= 100 {
		t.Fatalf("expected fallback penalty, got %#v", measurement.Quality)
	}
	if want := "excessive_fallback_usage"; !contains(measurement.Quality.Issues, want) {
		t.Fatalf("expected %q issue, got %#v", want, measurement.Quality)
	}
}

func TestSummarizeDurations(t *testing.T) {
	stats := bench.NewHarness(12)
	measurement, err := stats.Benchmark(bench.Fixture{
		Name:        "durations",
		Class:       "edge",
		ProfileName: "generic-summary",
		Execution:   engine.Execution{Stdout: "one\ntwo\nthree"},
		Invocation:  engine.Invocation{Display: []string{"summary", "tail"}},
	}, 3)
	if err != nil {
		t.Fatalf("benchmark durations: %v", err)
	}
	if measurement.Durations.Samples != 3 {
		t.Fatalf("unexpected duration sample count: %#v", measurement.Durations)
	}
	if measurement.Durations.Total < measurement.Durations.Min {
		t.Fatalf("unexpected duration total: %#v", measurement.Durations)
	}
	if measurement.Durations.Max < measurement.Durations.P95 || measurement.Durations.P95 < measurement.Durations.P50 {
		t.Fatalf("unexpected percentile ordering: %#v", measurement.Durations)
	}
	if measurement.Duration < 0 || measurement.Durations.Total < 0*time.Nanosecond {
		t.Fatalf("expected non-negative durations: %#v", measurement.Durations)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
