package bench_test

import (
	"strings"
	"testing"

	"szr/internal/bench"
	"szr/internal/engine"
)

func TestFixturesAndHarness(t *testing.T) {
	fixtures, err := bench.Fixtures()
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	if len(fixtures) != 7 {
		t.Fatalf("expected 7 fixtures, got %d", len(fixtures))
	}

	specs := bench.Specs()
	if len(specs) != len(fixtures) {
		t.Fatalf("spec count mismatch: %d vs %d", len(specs), len(fixtures))
	}
	specs[0].Name = "mutated"
	if bench.Specs()[0].Name == "mutated" {
		t.Fatal("specs should be cloned")
	}

	mustFixtures := bench.MustFixtures()
	if len(mustFixtures) != len(fixtures) {
		t.Fatalf("must fixtures count mismatch: %d vs %d", len(mustFixtures), len(fixtures))
	}

	harness := bench.NewHarness(12)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.RawCombined() == "" {
				t.Fatal("expected non-empty raw output")
			}

			rendered, err := harness.Render(fixture)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if strings.TrimSpace(rendered) == "" {
				t.Fatal("expected rendered output")
			}

			measurement, err := harness.Measure(fixture)
			if err != nil {
				t.Fatalf("measure: %v", err)
			}
			if measurement.Duration < 0 {
				t.Fatalf("unexpected negative duration: %v", measurement.Duration)
			}
			if measurement.Durations.Samples != 9 {
				t.Fatalf("expected 9 timing samples, got %#v", measurement.Durations)
			}
			if measurement.Durations.Min > measurement.Durations.P50 || measurement.Durations.P50 > measurement.Durations.P95 || measurement.Durations.P95 > measurement.Durations.Max {
				t.Fatalf("unexpected duration ordering: %#v", measurement.Durations)
			}
			if measurement.ProfileName != fixture.ProfileName || measurement.FixtureName != fixture.Name {
				t.Fatalf("unexpected measurement identity: %#v", measurement)
			}
			if measurement.RawBytes <= 0 || measurement.RawTokens <= 0 || measurement.ParsedBytes <= 0 {
				t.Fatalf("expected positive raw metrics: %#v", measurement)
			}
			if measurement.FilteredBytes <= 0 || measurement.FilteredTokens <= 0 || measurement.EmittedBytes <= 0 {
				t.Fatalf("expected positive filtered metrics: %#v", measurement)
			}
			if measurement.TokenSavingsPct <= 0 {
				t.Fatalf("expected token savings for %s, got %#v", fixture.Name, measurement)
			}
			if measurement.CommandFingerprint == "" {
				t.Fatalf("expected command fingerprint: %#v", measurement)
			}
			if measurement.Quality.Score < fixture.MinQualityScore {
				t.Fatalf("expected quality score >= %d for %s, got %#v", fixture.MinQualityScore, fixture.Name, measurement.Quality)
			}
			if !measurement.Expectation.OK {
				t.Fatalf("expected measurement expectation to pass for %s, got %#v", fixture.Name, measurement.Expectation)
			}
			for _, fragment := range fixture.ExpectedContains {
				if !strings.Contains(measurement.Rendered, fragment) {
					t.Fatalf("expected fragment %q in rendered output %q", fragment, measurement.Rendered)
				}
			}
		})
	}
}

func TestHarnessEdgeCases(t *testing.T) {
	harness := bench.NewHarnessWithProfiles([]engine.Profile{{
		Name: "blank",
		Render: func(engine.Invocation, engine.Execution) string {
			return ""
		},
	}})

	rendered, err := harness.Render(bench.Fixture{
		Name:        "fallback",
		Class:       "edge",
		ProfileName: "blank",
		Execution: engine.Execution{
			Stdout: "raw-only",
		},
	})
	if err != nil {
		t.Fatalf("render fallback: %v", err)
	}
	if rendered != "raw-only" {
		t.Fatalf("expected raw fallback, got %q", rendered)
	}

	measurement, err := harness.Measure(bench.Fixture{
		Name:        "empty",
		Class:       "edge",
		ProfileName: "blank",
	})
	if err != nil {
		t.Fatalf("measure empty: %v", err)
	}
	if measurement.ByteRatio != 0 || measurement.TokenRatio != 0 || measurement.ByteSavingsPct != 0 || measurement.TokenSavingsPct != 0 {
		t.Fatalf("expected zero ratios for empty measurement, got %#v", measurement)
	}
	if measurement.Quality.Score != 40 {
		t.Fatalf("expected empty quality penalty, got %#v", measurement.Quality)
	}
	if len(measurement.Quality.Issues) == 0 || measurement.Quality.Issues[0] != "zero_actionable_lines" {
		t.Fatalf("expected zero actionable line issue, got %#v", measurement.Quality)
	}

	if _, ok := harness.Profile("missing"); ok {
		t.Fatal("missing profile should not resolve")
	}
	if _, err := harness.Render(bench.Fixture{ProfileName: "missing"}); err == nil {
		t.Fatal("expected unknown profile error")
	}
}
