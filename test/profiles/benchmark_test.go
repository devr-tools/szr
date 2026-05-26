package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
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
			benchProfileComparison(b, len(bench.exec.Stdout)+len(bench.exec.Stderr),
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
			benchProfileComparison(b, len(bench.stdout)+len(bench.stderr),
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

func benchProfileComparison(b *testing.B, inputBytes int, current func() string, legacy func() string) {
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
			for i := 0; i < b.N; i++ {
				if got := candidate.fn(); got == "" {
					b.Fatal("expected profile output")
				}
			}
		})
	}
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
