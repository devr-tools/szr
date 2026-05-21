package test

import (
	"errors"
	"strings"
	"testing"

	"szr/internal/bench"
	"szr/internal/engine"
)

func TestBenchmarkFixturesMeasure(t *testing.T) {
	fixtures, err := bench.Fixtures()
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	if len(fixtures) != 5 {
		t.Fatalf("expected 5 fixtures, got %d", len(fixtures))
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
			if measurement.ProfileName != fixture.ProfileName || measurement.FixtureName != fixture.Name {
				t.Fatalf("unexpected measurement identity: %#v", measurement)
			}
			if measurement.RawBytes <= 0 || measurement.RawTokens <= 0 {
				t.Fatalf("expected positive raw metrics: %#v", measurement)
			}
			if measurement.FilteredBytes <= 0 || measurement.FilteredTokens <= 0 {
				t.Fatalf("expected positive filtered metrics: %#v", measurement)
			}
			if measurement.TokenSavingsPct <= 0 {
				t.Fatalf("expected token savings for %s, got %#v", fixture.Name, measurement)
			}
			for _, fragment := range fixture.ExpectedContains {
				if !strings.Contains(measurement.Rendered, fragment) {
					t.Fatalf("expected fragment %q in rendered output %q", fragment, measurement.Rendered)
				}
			}
		})
	}
}

func TestBenchmarkHarnessEdgeCases(t *testing.T) {
	harness := bench.NewHarnessWithProfiles([]engine.Profile{
		{
			Name: "blank",
			Render: func(engine.Invocation, engine.Execution) string {
				return ""
			},
		},
	})

	fixture := bench.Fixture{
		Name:        "fallback",
		Class:       "edge",
		ProfileName: "blank",
		Execution: engine.Execution{
			Stdout: "raw-only",
		},
	}
	rendered, err := harness.Render(fixture)
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

	if _, ok := harness.Profile("missing"); ok {
		t.Fatal("missing profile should not resolve")
	}
	if _, err := harness.Render(bench.Fixture{ProfileName: "missing"}); err == nil {
		t.Fatal("expected unknown profile error")
	}
}

func TestLoadBenchmarkFixturesErrors(t *testing.T) {
	_, err := bench.LoadFixtures(nil, bench.Specs())
	if err == nil || !strings.Contains(err.Error(), "missing fixture reader") {
		t.Fatalf("expected missing reader error, got %v", err)
	}

	readErr := errors.New("boom")
	_, err = bench.LoadFixtures(func(string) ([]byte, error) {
		return nil, readErr
	}, []bench.Spec{{
		Name:       "broken",
		StdoutFile: "testdata/missing.txt",
	}})
	if err == nil || !strings.Contains(err.Error(), "testdata/missing.txt") {
		t.Fatalf("expected wrapped read error, got %v", err)
	}
}
