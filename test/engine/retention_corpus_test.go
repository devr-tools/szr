package engine_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/bench"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
)

// TestRetentionVerifierNoFalsePositivesOnBenchCorpus sweeps every internal
// bench fixture through the verifier exactly as the pipeline would: the
// built-in profiles' renders must already retain every critical fact, so the
// verifier must queue zero repairs. A repair here would mean the verifier
// second-guesses renders the fidelity gates already vouch for.
func TestRetentionVerifierNoFalsePositivesOnBenchCorpus(t *testing.T) {
	t.Parallel()

	harness := bench.NewHarness(12)
	for _, fixture := range bench.MustFixtures() {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()

			raw := fixture.RawCombined()
			if history.EstimateTokens(raw) < 200 {
				t.Skipf("below verifier arming threshold")
			}
			rendered, err := harness.Render(fixture)
			if err != nil {
				t.Fatalf("render fixture: %v", err)
			}
			report := engine.VerifyRetention(raw, rendered, fixture.Execution.ExitCode != 0)
			if len(report.MissingLines) != 0 {
				t.Fatalf("verifier flagged dropped needles %q (lines %q) in a vetted render:\n%s", report.MissingNeedles, report.MissingLines, rendered)
			}
		})
	}
}

// BenchmarkRetentionVerify measures the extraction+verification pass the
// verifier adds per emission. The p50 must stay well under a millisecond for
// typical outputs.
func BenchmarkRetentionVerify(b *testing.B) {
	failing := buildRetentionBenchFailingOutput(400)
	passing := buildRetentionBenchPassingOutput(400)
	render := "run summary: 3 problems in module replayd; see anchors below\n" +
		"src/replay/buffer.rs:214 E0599 commit_frame missing\n" +
		"3 failed, 37 passed"

	cases := []struct {
		name string
		raw  string
		exit bool
	}{
		{name: "failing-400-lines", raw: failing, exit: true},
		{name: "passing-400-lines", raw: passing, exit: false},
	}
	for _, bc := range cases {
		bc := bc
		b.Run(bc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bc.raw)))
			for i := 0; i < b.N; i++ {
				engine.VerifyRetention(bc.raw, render, bc.exit)
			}
		})
	}
}

func buildRetentionBenchFailingOutput(lines int) string {
	parts := make([]string, 0, lines+4)
	for i := 0; i < lines; i++ {
		parts = append(parts, "compile unit "+threeDigitIndex(i)+" processed in "+itoaBenchmark(i%9)+"ms with cache reuse enabled")
	}
	parts = append(parts,
		"error[E0599]: no method named `commit_frame` found for struct `ReplayBuffer`",
		"   --> src/replay/buffer.rs:214:31",
		"FAILED tests/test_auth.py::test_login_rejects_bad_token - assert 200 == 401",
		"3 failed, 37 passed",
	)
	return strings.Join(parts, "\n")
}

func buildRetentionBenchPassingOutput(lines int) string {
	parts := make([]string, 0, lines+1)
	for i := 0; i < lines; i++ {
		parts = append(parts, "stage "+threeDigitIndex(i)+" ok duration="+itoaBenchmark(i%40)+"ms artifacts=3 cache=warm")
	}
	parts = append(parts, "40 passed, 0 failed")
	return strings.Join(parts, "\n")
}
