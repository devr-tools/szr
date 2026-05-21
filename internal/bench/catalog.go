package bench

type Spec struct {
	Name             string
	Class            string
	Description      string
	ProfileName      string
	Command          []string
	Display          []string
	Cwd              string
	ExitCode         int
	StdoutFile       string
	StderrFile       string
	ExpectedContains []string
	MinTokenSavings  float64
	MinQualityScore  int
}

func Specs() []Spec {
	return append([]Spec(nil), builtinSpecs...)
}

var builtinSpecs = []Spec{
	{
		Name:        "clean-pass",
		Class:       "clean-pass",
		Description: "Structured go test success output with multiple passing packages.",
		ProfileName: "go-test-json",
		Command:     []string{"go", "test", "./..."},
		Display:     []string{"go", "test", "./..."},
		StdoutFile:  "testdata/go_test_clean.jsonl",
		ExpectedContains: []string{
			"packages: pass=3 fail=0",
			"all tests passed",
		},
		MinTokenSavings: 60,
		MinQualityScore: 95,
	},
	{
		Name:        "noisy-fail",
		Class:       "noisy-fail",
		Description: "Unstructured test failure output with passing noise, stack traces, and tool errors.",
		ProfileName: "generic-test",
		Command:     []string{"test", "npm", "run", "test", "--", "--runInBand"},
		Display:     []string{"test", "npm", "run", "test"},
		ExitCode:    1,
		StdoutFile:  "testdata/noisy_fail.txt",
		ExpectedContains: []string{
			"button.test.tsx > Button > renders primary state",
			"Error: expected primary button to be visible",
			"error Command failed with exit code 1.",
		},
		MinTokenSavings: 20,
		MinQualityScore: 80,
	},
	{
		Name:        "diff-stat",
		Class:       "diff-stat-output",
		Description: "Mixed patch and stat-heavy git diff output across multiple files.",
		ProfileName: "git-diff",
		Command:     []string{"git", "diff"},
		Display:     []string{"git", "diff"},
		StdoutFile:  "testdata/git_diff.txt",
		ExpectedContains: []string{
			"files=2",
			"cmd/szr/main.go",
			"2 files changed, 13 insertions(+), 4 deletions(-)",
		},
		MinTokenSavings: 15,
		MinQualityScore: 70,
	},
	{
		Name:        "giant-diff",
		Class:       "giant-diff",
		Description: "Large git diff with many files and stat lines that should collapse to a stable preview.",
		ProfileName: "git-diff",
		Command:     []string{"git", "diff", "--stat"},
		Display:     []string{"git", "diff"},
		StdoutFile:  "testdata/giant_diff.txt",
		ExpectedContains: []string{
			"files=12",
			"internal/parser/lexer.go",
			"internal/parser/token.go",
		},
		MinTokenSavings: 50,
		MinQualityScore: 75,
	},
	{
		Name:        "repeated-logs",
		Class:       "repeated-logs",
		Description: "Long repeated application logs that should collapse to a shallow preview.",
		ProfileName: "generic-summary",
		Command:     []string{"summary", "tail", "-n", "200", "var/log/build.log"},
		Display:     []string{"summary", "tail", "-n", "200", "var/log/build.log"},
		StdoutFile:  "testdata/repeated_logs.txt",
		ExpectedContains: []string{
			"2026-05-20T21:00:00Z INFO worker.1 step=compile status=running",
			"... +8 more lines",
		},
		MinTokenSavings: 30,
		MinQualityScore: 70,
	},
	{
		Name:        "compiler-diagnostics",
		Class:       "compiler-diagnostics",
		Description: "Go compiler output with download chatter on stdout and errors on stderr.",
		ProfileName: "go-build",
		Command:     []string{"go", "build", "./..."},
		Display:     []string{"go", "build", "./..."},
		ExitCode:    1,
		StdoutFile:  "testdata/compiler_diagnostics.stdout.txt",
		StderrFile:  "testdata/compiler_diagnostics.stderr.txt",
		ExpectedContains: []string{
			"internal/engine/runner.go:18:2: error: undefined: missingSymbol",
			"warning: deprecated helper benchmarkHarness will be removed",
		},
		MinTokenSavings: 10,
		MinQualityScore: 80,
	},
	{
		Name:        "mixed-stdio-fail",
		Class:       "mixed-stdout-stderr-fail",
		Description: "Failure that spreads actionable identifiers across stdout and stderr with summary noise.",
		ProfileName: "generic-test",
		Command:     []string{"test", "pnpm", "exec", "playwright", "test"},
		Display:     []string{"test", "pnpm", "exec", "playwright", "test"},
		ExitCode:    1,
		StdoutFile:  "testdata/mixed_stdio_fail.stdout.txt",
		StderrFile:  "testdata/mixed_stdio_fail.stderr.txt",
		ExpectedContains: []string{
			"FAIL tests/e2e/checkout.spec.ts > checkout flow > submits order",
			"TimeoutError: locator.click: Timeout 5000ms exceeded.",
			"Error: worker exited with code 1",
		},
		MinTokenSavings: 20,
		MinQualityScore: 85,
	},
}
