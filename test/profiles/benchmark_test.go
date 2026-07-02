package profiles_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
)

var (
	benchmarkGenericSummaryInput = strings.Join([]string{
		"line-01",
		"line-02",
		"line-03",
		"line-04",
		"line-05",
		"line-06",
		"line-07",
		"line-08",
		"line-09",
		"line-10",
	}, "\n")
	benchmarkGHRunListInput = strings.Join([]string{
		"completed\tsuccess\tbuild\tmain\tpush\t123456789\t20s\t2026-05-26T09:00:00Z",
		"completed\tfailure\ttest\tmain\tpush\t123456788\t31s\t2026-05-26T08:59:00Z",
		"queued\t\tdeploy\tmain\tworkflow_dispatch\t123456787\t0s\t2026-05-26T08:58:00Z",
		"completed\tsuccess\tlint\tfeature/refactor\tpull_request\t123456786\t18s\t2026-05-26T08:57:00Z",
		"completed\tsuccess\tsmoke\tfeature/refactor\tpull_request\t123456785\t27s\t2026-05-26T08:56:00Z",
		"completed\tsuccess\trelease\trelease/1.2\tpush\t123456784\t40s\t2026-05-26T08:55:00Z",
	}, "\n")
	benchmarkKubectlTopInput = strings.Join([]string{
		"NAME\tCPU(cores)\tMEMORY(bytes)",
		"api-7b9d5bdf44-h2p8z\t12m\t128Mi",
		"worker-6fd9d7f56d-9qjpl\t250m\t512Mi",
		"cache-0\t8m\t64Mi",
		"cron-28937412-zr8mq\t3m\t32Mi",
		"ingest-6848d8b5db-kr2rp\t180m\t412Mi",
	}, "\n")
	benchmarkGHRunListLongInput       = buildGHRunListLongInput(30)
	benchmarkKubectlTopLongInput      = buildKubectlTopLongInput(30)
	benchmarkCompactStreamStdoutHeavy = strings.Join([]string{
		"out-01",
		"out-02",
		"out-03",
		"out-04",
		"out-05",
		"out-06",
		"out-07",
		"out-08",
	}, "\n")
	benchmarkCompactStreamStderrHeavy = strings.Join([]string{
		"err-01",
		"err-02",
		"err-03",
		"err-04",
		"err-05",
		"err-06",
		"err-07",
		"err-08",
	}, "\n")
	benchmarkCompactStreamMixedStdout = strings.Join([]string{
		"out-01",
		"out-02",
		"out-03",
	}, "\n")
	benchmarkCompactStreamMixedStderr = strings.Join([]string{
		"err-01",
		"err-02",
		"err-03",
		"err-04",
		"err-05",
	}, "\n")
)

func BenchmarkDeclarativeProfileRender(b *testing.B) {
	list := profiles.Builtins(6)
	benches := []struct {
		name    string
		profile string
		inv     engine.Invocation
		exec    engine.Execution
	}{
		{
			name:    "generic-summary",
			profile: "generic-summary",
			inv:     engine.Invocation{Display: []string{"summary"}},
			exec:    engine.Execution{Stdout: benchmarkGenericSummaryInput},
		},
		{
			name:    "gh-run-list",
			profile: "gh-run-list",
			inv:     engine.Invocation{Display: []string{"gh", "run", "list"}},
			exec:    engine.Execution{Stdout: benchmarkGHRunListInput},
		},
		{
			name:    "kubectl-top",
			profile: "kubectl-top",
			inv:     engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			exec:    engine.Execution{Stdout: benchmarkKubectlTopInput},
		},
	}

	for _, bench := range benches {
		profile := findProfileForBenchmark(b, list, bench.profile)
		b.Run(bench.name, func(b *testing.B) {
			benchProfileComparison(b, len(bench.exec.Stdout)+len(bench.exec.Stderr), bench.exec.Stdout+"\n"+bench.exec.Stderr,
				func() string {
					return profile.Render(bench.inv, bench.exec)
				},
				func() string {
					return legacyProfileRender(bench.exec.Stdout, bench.exec.Stderr, 6)
				},
			)
		})
	}
}

func BenchmarkDeclarativeProfileStreamRender(b *testing.B) {
	list := profiles.Builtins(6)
	benches := []struct {
		name    string
		profile string
		inv     engine.Invocation
		stdout  string
		stderr  string
	}{
		{
			name:    "generic-summary",
			profile: "generic-summary",
			inv:     engine.Invocation{Display: []string{"summary"}},
			stdout:  benchmarkGenericSummaryInput,
		},
		{
			name:    "gh-run-list",
			profile: "gh-run-list",
			inv:     engine.Invocation{Display: []string{"gh", "run", "list"}},
			stdout:  benchmarkGHRunListInput,
		},
		{
			name:    "kubectl-top",
			profile: "kubectl-top",
			inv:     engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			stdout:  benchmarkKubectlTopInput,
		},
	}

	for _, bench := range benches {
		profile := findProfileForBenchmark(b, list, bench.profile)
		b.Run(bench.name, func(b *testing.B) {
			benchProfileComparison(b, len(bench.stdout)+len(bench.stderr), bench.stdout+"\n"+bench.stderr,
				func() string {
					reducer := profile.StreamRender(bench.inv, profile.Budget)
					reducer.ConsumeStdout([]byte(bench.stdout))
					if bench.stderr != "" {
						reducer.ConsumeStderr([]byte(bench.stderr))
					}
					return reducer.Result()
				},
				func() string {
					return legacyProfileStreamRender(bench.stdout, bench.stderr, 6)
				},
			)
		})
	}
}

func BenchmarkDeclarativeProfileStreamRenderChunked(b *testing.B) {
	list := profiles.Builtins(6)
	benches := []struct {
		name       string
		profile    string
		inv        engine.Invocation
		stdout     string
		stderr     string
		chunkStyle string
	}{
		{
			name:       "gh-run-list/fixed-32",
			profile:    "gh-run-list",
			inv:        engine.Invocation{Display: []string{"gh", "run", "list"}},
			stdout:     benchmarkGHRunListInput,
			chunkStyle: "fixed-32",
		},
		{
			name:       "gh-run-list/line",
			profile:    "gh-run-list",
			inv:        engine.Invocation{Display: []string{"gh", "run", "list"}},
			stdout:     benchmarkGHRunListInput,
			chunkStyle: "line",
		},
		{
			name:       "kubectl-top/fixed-32",
			profile:    "kubectl-top",
			inv:        engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			stdout:     benchmarkKubectlTopInput,
			chunkStyle: "fixed-32",
		},
		{
			name:       "kubectl-top/line",
			profile:    "kubectl-top",
			inv:        engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			stdout:     benchmarkKubectlTopInput,
			chunkStyle: "line",
		},
	}

	for _, bench := range benches {
		profile := findProfileForBenchmark(b, list, bench.profile)
		b.Run(bench.name, func(b *testing.B) {
			benchProfileComparison(b, len(bench.stdout)+len(bench.stderr), bench.stdout+"\n"+bench.stderr,
				func() string {
					reducer := profile.StreamRender(bench.inv, profile.Budget)
					feedReducerChunks(reducer, bench.stdout, bench.stderr, bench.chunkStyle)
					return reducer.Result()
				},
				func() string {
					reducer := filters.NewCompactLineReducer(6, 0)
					feedReducerChunks(reducer, bench.stdout, bench.stderr, bench.chunkStyle)
					return reducer.Result()
				},
			)
		})
	}
}

func BenchmarkDeclarativeProfileTokenSavings(b *testing.B) {
	list := profiles.Builtins(6)
	benches := []struct {
		name    string
		profile string
		inv     engine.Invocation
		exec    engine.Execution
	}{
		{
			name:    "gh-run-list-long",
			profile: "gh-run-list",
			inv:     engine.Invocation{Display: []string{"gh", "run", "list"}},
			exec:    engine.Execution{Stdout: benchmarkGHRunListLongInput},
		},
		{
			name:    "kubectl-top-long",
			profile: "kubectl-top",
			inv:     engine.Invocation{Display: []string{"kubectl", "top", "pods"}},
			exec:    engine.Execution{Stdout: benchmarkKubectlTopLongInput},
		},
	}

	for _, bench := range benches {
		profile := findProfileForBenchmark(b, list, bench.profile)
		b.Run(bench.name, func(b *testing.B) {
			benchProfileComparison(b, len(bench.exec.Stdout)+len(bench.exec.Stderr), bench.exec.Stdout+"\n"+bench.exec.Stderr,
				func() string {
					return profile.Render(bench.inv, bench.exec)
				},
				func() string {
					return legacyProfileRender(bench.exec.Stdout, bench.exec.Stderr, 6)
				},
			)
		})
	}
}

func BenchmarkCompactLinesTwoStreamComparison(b *testing.B) {
	cases := []struct {
		name   string
		stdout string
		stderr string
	}{
		{name: "stdout-heavy", stdout: benchmarkCompactStreamStdoutHeavy, stderr: "err-01"},
		{name: "stderr-heavy", stdout: "out-01", stderr: benchmarkCompactStreamStderrHeavy},
		{name: "mixed", stdout: benchmarkCompactStreamMixedStdout, stderr: benchmarkCompactStreamMixedStderr},
	}

	for _, bench := range cases {
		b.Run(bench.name, func(b *testing.B) {
			benchProfileComparison(b, len(bench.stdout)+len(bench.stderr), bench.stdout+"\n"+bench.stderr,
				func() string {
					reducer := filters.NewDeclarativeBuiltinReducer("compact_lines", "lines", 6, true, true)
					reducer.ConsumeStdout([]byte(bench.stdout))
					reducer.ConsumeStderr([]byte(bench.stderr))
					return reducer.Result()
				},
				func() string {
					reducer := filters.NewCompactLineReducer(6, 0)
					reducer.ConsumeStdout([]byte(bench.stdout))
					reducer.ConsumeStderr([]byte(bench.stderr))
					return reducer.Result()
				},
			)
		})
	}
}

func benchProfileComparison(b *testing.B, inputBytes int, inputText string, current func() string, legacy func() string) {
	b.Helper()
	for _, candidate := range []struct {
		name string
		fn   func() string
	}{
		{name: "declarative", fn: current},
		{name: "legacy", fn: legacy},
	} {
		b.Run(candidate.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(inputBytes))
			sample := candidate.fn()
			reportApproxTokenSavings(b, inputText, sample)
			for i := 0; i < b.N; i++ {
				if got := candidate.fn(); got == "" {
					b.Fatal("expected profile output")
				}
			}
		})
	}
}

type streamConsumer interface {
	ConsumeStdout([]byte)
	ConsumeStderr([]byte)
}

func feedReducerChunks(reducer streamConsumer, stdout string, stderr string, chunkStyle string) {
	feedStream := func(consume func([]byte), input string) {
		switch chunkStyle {
		case "line":
			lines := strings.Split(input, "\n")
			for i, line := range lines {
				if i < len(lines)-1 {
					consume([]byte(line + "\n"))
					continue
				}
				if line != "" {
					consume([]byte(line))
				}
			}
		case "fixed-32":
			for start := 0; start < len(input); start += 32 {
				end := start + 32
				if end > len(input) {
					end = len(input)
				}
				consume([]byte(input[start:end]))
			}
		default:
			if input != "" {
				consume([]byte(input))
			}
		}
	}

	if stdout != "" {
		feedStream(reducer.ConsumeStdout, stdout)
	}
	if stderr != "" {
		feedStream(reducer.ConsumeStderr, stderr)
	}
}

func reportApproxTokenSavings(b *testing.B, input string, output string) {
	inputTokens := history.EstimateTokens(input)
	outputTokens := history.EstimateTokens(output)
	if inputTokens <= 0 {
		return
	}
	retained := (float64(outputTokens) / float64(inputTokens)) * 100
	saved := 100 - retained
	b.ReportMetric(retained, "tokens_retained_pct")
	b.ReportMetric(saved, "tokens_saved_pct")
}

func buildGHRunListLongInput(rows int) string {
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

func buildKubectlTopLongInput(rows int) string {
	lines := make([]string, 0, rows+1)
	lines = append(lines, "NAME\tCPU(cores)\tMEMORY(bytes)")
	for i := 0; i < rows; i++ {
		lines = append(lines, strings.Join([]string{
			"pod-" + twoDigitIndex(i),
			itoaTest(10+i) + "m",
			itoaTest(128+i*8) + "Mi",
		}, "\t"))
	}
	return strings.Join(lines, "\n")
}

func twoDigitIndex(i int) string {
	if i < 10 {
		return "0" + itoaTest(i)
	}
	return itoaTest(i)
}

func itoaTest(i int) string {
	return strconv.Itoa(i)
}

func legacyProfileRender(stdout string, stderr string, maxLines int) string {
	return legacyProfileCompactLines(stdout+"\n"+stderr, maxLines)
}

func legacyProfileStreamRender(stdout string, stderr string, maxLines int) string {
	reducer := filters.NewCompactLineReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(stdout))
	if stderr != "" {
		reducer.ConsumeStderr([]byte(stderr))
	}
	return reducer.Result()
}

func legacyProfileCompactLines(input string, maxLines int) string {
	reducer := filters.NewCompactLineReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func findProfileForBenchmark(b *testing.B, list []engine.Profile, name string) engine.Profile {
	b.Helper()
	for _, profile := range list {
		if profile.Name == name {
			return profile
		}
	}
	b.Fatalf("missing profile %q", name)
	return engine.Profile{}
}
