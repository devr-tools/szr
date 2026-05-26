package filters_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
)

var (
	benchmarkCompactLinesInput = strings.Join([]string{
		"workflow\tstatus\tbranch\tupdated",
		"build\tcompleted\tmain\t1m",
		"test\tcompleted\tmain\t2m",
		"lint\tcompleted\tmain\t3m",
		"deploy\tqueued\tmain\t4m",
		"preview\tcompleted\tfeature/refactor\t5m",
		"smoke\tcompleted\tfeature/refactor\t6m",
		"release\tcompleted\trelease/1.2\t7m",
		"rollback\tcompleted\thotfix\t8m",
		"cleanup\tcompleted\tmain\t9m",
		"audit\tcompleted\tmain\t10m",
		"sync\tcompleted\tmain\t11m",
	}, "\n")
	benchmarkReadMinimalInput = strings.Join([]string{
		"// package docs",
		"package preview",
		"",
		"// Config describes the preview contract.",
		"type Config struct {",
		"    Name string",
		"    Path string",
		"}",
		"",
		"# generated heading should be dropped",
		"func Render() string { return \"ready\" }",
	}, "\n")
)

func BenchmarkCompactLinesComparison(b *testing.B) {
	benchComparison(b, "compact_lines", benchmarkCompactLinesInput,
		func() string {
			return filters.RenderDeclarativeBuiltin("compact_lines", benchmarkCompactLinesInput, 6)
		},
		func() string {
			return legacyCompactLines(benchmarkCompactLinesInput, 6)
		},
	)
}

func BenchmarkCompactLineReducerComparison(b *testing.B) {
	benchComparison(b, "compact_lines_stream", benchmarkCompactLinesInput,
		func() string {
			reducer := filters.NewDeclarativeBuiltinReducer("compact_lines", "lines", 6, true, false)
			reducer.ConsumeStdout([]byte(benchmarkCompactLinesInput))
			return reducer.Result()
		},
		func() string {
			reducer := filters.NewCompactLineReducer(6, 0)
			reducer.ConsumeStdout([]byte(benchmarkCompactLinesInput))
			return reducer.Result()
		},
	)
}

func BenchmarkReadLevelMinimalComparison(b *testing.B) {
	benchComparison(b, "read_minimal", benchmarkReadMinimalInput,
		func() string {
			return filters.ReadLevel([]byte(benchmarkReadMinimalInput), "minimal", false, 6)
		},
		func() string {
			return legacyReadLevelMinimal(benchmarkReadMinimalInput, 6)
		},
	)
}

func benchComparison(b *testing.B, name string, input string, current func() string, legacy func() string) {
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
			b.SetBytes(int64(len(input)))
			for i := 0; i < b.N; i++ {
				if got := candidate.fn(); got == "" {
					b.Fatalf("expected %s output", name)
				}
			}
		})
	}
}

func legacyCompactLines(input string, maxLines int) string {
	reducer := filters.NewCompactLineReducer(maxLines, 0)
	reducer.ConsumeStdout([]byte(input))
	return reducer.Result()
}

func legacyReadLevelMinimal(input string, maxLines int) string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		filtered = append(filtered, raw)
	}
	if maxLines > 0 && len(filtered) > maxLines {
		filtered = append(filtered[:maxLines], fmt.Sprintf("... +%d more lines", len(filtered)-maxLines))
	}
	return strings.Join(filtered, "\n")
}
