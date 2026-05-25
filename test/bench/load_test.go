package bench_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/bench"
)

func TestLoadFixturesErrors(t *testing.T) {
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

func TestMustLoadFixturesAndMeasureErrors(t *testing.T) {
	fixtures := bench.MustLoadFixtures(func() ([]bench.Fixture, error) {
		return []bench.Fixture{{Name: "ok"}}, nil
	})
	if len(fixtures) != 1 || fixtures[0].Name != "ok" {
		t.Fatalf("unexpected must load fixtures: %#v", fixtures)
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected MustLoadFixtures to panic on loader error")
		}
	}()
	_ = bench.MustLoadFixtures(func() ([]bench.Fixture, error) {
		return nil, errors.New("boom")
	})
}

func TestLoadFixtureReadErrorsAndMissingProfileMeasure(t *testing.T) {
	_, err := bench.LoadFixtures(func(name string) ([]byte, error) {
		if strings.Contains(name, "stdout") {
			return []byte("ok"), nil
		}
		return nil, errors.New("stderr boom")
	}, []bench.Spec{{
		Name:       "broken",
		StdoutFile: "testdata/stdout.txt",
		StderrFile: "testdata/stderr.txt",
	}})
	if err == nil || !strings.Contains(err.Error(), "stderr.txt") {
		t.Fatalf("expected stderr read error, got %v", err)
	}

	harness := bench.NewHarnessWithProfiles(nil)
	if _, err := harness.Measure(bench.Fixture{ProfileName: "missing"}); err == nil {
		t.Fatal("expected missing profile measure error")
	}
}
