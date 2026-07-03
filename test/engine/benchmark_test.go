package engine_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
)

var (
	benchmarkEngineGenericSummaryLongInput = buildEngineGenericSummaryLongInput(60)
	benchmarkEngineGHRunListLongInput      = buildEngineGHRunListLongInput(30)
	// 60 rows keeps this fixture above the compression contract's raw-token
	// threshold (compressionContractMinRawTokens) so the contract tests stay
	// observable.
	benchmarkEngineKubectlTopLongInput = buildEngineKubectlTopLongInput(60)
)

func BenchmarkEngineRenderTokenSavings(b *testing.B) {
	list := profiles.Builtins(6)
	benches := []struct {
		name    string
		profile string
		inv     engine.Invocation
		exec    engine.Execution
	}{
		{
			name:    "generic-summary-long",
			profile: "generic-summary",
			inv:     engine.Invocation{Display: []string{"summary"}},
			exec:    engine.Execution{Stdout: benchmarkEngineGenericSummaryLongInput},
		},
		{
			name:    "gh-run-list-long",
			profile: "gh-run-list",
			inv:     engine.Invocation{Display: []string{"gh", "run", "list"}},
			exec:    engine.Execution{Stdout: benchmarkEngineGHRunListLongInput},
		},
		{
			name:    "kubectl-top-long",
			profile: "kubectl-top",
			inv:     engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			exec:    engine.Execution{Stdout: benchmarkEngineKubectlTopLongInput},
		},
	}

	for _, bench := range benches {
		profile := findEngineBenchmarkProfile(b, list, bench.profile)
		rawCombined := bench.exec.Stdout + bench.exec.Stderr
		b.Run(bench.name, func(b *testing.B) {
			for _, mode := range []struct {
				name string
				adv  config.Advanced
			}{
				{
					name: "contract-on",
					adv:  config.Advanced{CompressionContract: true, CompactArtifactRefs: true},
				},
				{
					name: "contract-off",
					adv:  config.Advanced{CompressionContract: false, CompactArtifactRefs: true},
				},
			} {
				b.Run(mode.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(rawCombined)))
					inv := bench.inv
					inv.Advanced = mode.adv
					sample := engine.RenderExecution(profile, inv, bench.exec, 6, false)
					reportEngineTokenSavings(b, sample.RawCombined, sample.Text)
					for i := 0; i < b.N; i++ {
						if got := engine.RenderExecution(profile, inv, bench.exec, 6, false); got.Text == "" {
							b.Fatal("expected rendered engine output")
						}
					}
				})
			}
		})
	}
}

func reportEngineTokenSavings(b *testing.B, input string, output string) {
	inputTokens := history.EstimateTokens(input)
	outputTokens := history.EstimateTokens(output)
	if inputTokens <= 0 {
		return
	}
	retained := (float64(outputTokens) / float64(inputTokens)) * 100
	saved := 100 - retained
	targetMet := 0.0
	if outputTokens <= retainedTokenCap(inputTokens) {
		targetMet = 1
	}
	b.ReportMetric(float64(inputTokens), "input_tokens")
	b.ReportMetric(float64(outputTokens), "output_tokens")
	b.ReportMetric(retained, "tokens_retained_pct")
	b.ReportMetric(saved, "tokens_saved_pct")
	b.ReportMetric(targetMet, "target_met")
}

func findEngineBenchmarkProfile(b testing.TB, list []engine.Profile, name string) engine.Profile {
	b.Helper()
	for _, profile := range list {
		if profile.Name == name {
			return profile
		}
	}
	b.Fatalf("missing profile %q", name)
	return engine.Profile{}
}

// retainedTokenCap mirrors the engine's compression contract: large outputs
// retain <=1/5 of their raw tokens, with a 48-token fidelity floor so small
// renders are never crushed below a usable diagnostic size.
func retainedTokenCap(rawTokens int) int {
	if rawTokens <= 0 {
		return 0
	}
	allowed := (rawTokens + 4) / 5
	if allowed < 48 {
		allowed = 48
	}
	return allowed
}

func buildEngineGenericSummaryLongInput(lines int) string {
	parts := make([]string, 0, lines)
	for i := 0; i < lines; i++ {
		parts = append(parts, "line-"+threeDigitIndex(i)+" build="+twoDigitIndex(i%17)+" status=ok duration="+itoaBenchmark(i+10)+"s")
	}
	return strings.Join(parts, "\n")
}

func buildEngineGHRunListLongInput(rows int) string {
	lines := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		status := "completed"
		conclusion := "success"
		event := "push"
		if i%5 == 1 {
			conclusion = "failure"
		}
		if i%7 == 2 {
			status = "queued"
			conclusion = ""
			event = "workflow_dispatch"
		}
		lines = append(lines, strings.Join([]string{
			status,
			conclusion,
			"workflow-" + twoDigitIndex(i),
			"feature/refactor-" + twoDigitIndex(i),
			event,
			"1234567" + twoDigitIndex(i),
			"2" + twoDigitIndex(i) + "s",
			"2026-05-26T08:" + twoDigitIndex(i) + ":00Z",
		}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func buildEngineKubectlTopLongInput(rows int) string {
	lines := make([]string, 0, rows+1)
	lines = append(lines, "NAME\tCPU(cores)\tMEMORY(bytes)")
	for i := 0; i < rows; i++ {
		lines = append(lines, strings.Join([]string{
			"pod-" + twoDigitIndex(i),
			itoaBenchmark(10+i) + "m",
			itoaBenchmark(128+i*8) + "Mi",
		}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func twoDigitIndex(i int) string {
	if i < 10 {
		return "0" + itoaBenchmark(i)
	}
	return itoaBenchmark(i)
}

func threeDigitIndex(i int) string {
	switch {
	case i < 10:
		return "00" + itoaBenchmark(i)
	case i < 100:
		return "0" + itoaBenchmark(i)
	default:
		return itoaBenchmark(i)
	}
}

func itoaBenchmark(i int) string {
	return strconv.Itoa(i)
}
