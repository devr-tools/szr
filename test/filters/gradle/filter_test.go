package gradle_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/history"

	gradlefilter "github.com/devr-tools/szr/internal/filters/gradle"
)

func gradleFailureOutput() string {
	return strings.Join([]string{
		"> Task :core:compileJava",
		"> Task :core:processResources UP-TO-DATE",
		"> Task :app:compileJava FAILED",
		"",
		"/workspace/app/src/main/java/dev/demo/OrderService.java:42: error: cannot find symbol",
		"        return repository.findByIdd(id);",
		"                         ^",
		"  symbol:   method findByIdd(long)",
		"1 error",
		"",
		"FAILURE: Build failed with an exception.",
		"",
		"* What went wrong:",
		"Execution failed for task ':app:compileJava'.",
		"> Compilation failed; see the compiler error output for details.",
		"",
		"* Try:",
		"> Run with --stacktrace option to get the stack trace.",
		"",
		"* Get more help at https://help.gradle.org.",
		"",
		"BUILD FAILED in 12s",
		"5 actionable tasks: 4 executed, 1 up-to-date",
	}, "\n")
}

func TestSummarizeGradleFailure(t *testing.T) {
	got := gradlefilter.SummarizeGradle(gradleFailureOutput(), 12)
	for _, want := range []string{
		"> Task :app:compileJava FAILED",
		"OrderService.java:42: error: cannot find symbol",
		"FAILURE: Build failed with an exception.",
		"Execution failed for task ':app:compileJava'.",
		"BUILD FAILED in 12s",
		"5 actionable tasks: 4 executed, 1 up-to-date",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gradle failure summary:\n%s", want, got)
		}
	}
	for _, drop := range []string{"> Run with --stacktrace", "* Get more help", "symbol:   method"} {
		if strings.Contains(got, drop) {
			t.Fatalf("expected %q to be dropped from gradle failure summary:\n%s", drop, got)
		}
	}
}

func TestSummarizeGradleTestFailure(t *testing.T) {
	input := strings.Join([]string{
		"> Task :app:test FAILED",
		"",
		"OrderServiceTest > refundsCancelledOrder FAILED",
		"    org.opentest4j.AssertionFailedError at OrderServiceTest.java:88",
		"",
		"12 tests completed, 2 failed",
		"",
		"BUILD FAILED in 34s",
	}, "\n")
	got := gradlefilter.SummarizeGradle(input, 12)
	for _, want := range []string{
		"OrderServiceTest > refundsCancelledOrder FAILED",
		"org.opentest4j.AssertionFailedError at OrderServiceTest.java:88",
		"12 tests completed, 2 failed",
		"BUILD FAILED in 34s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gradle test failure summary:\n%s", want, got)
		}
	}
}

func TestSummarizeGradleSuccessCompactsToCounts(t *testing.T) {
	lines := []string{}
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("> Task :app:task%02d", i))
	}
	lines = append(lines, "> Task :app:jar UP-TO-DATE", "", "BUILD SUCCESSFUL in 45s", "21 actionable tasks: 20 executed, 1 up-to-date")
	got := gradlefilter.SummarizeGradle(strings.Join(lines, "\n"), 12)

	for _, want := range []string{"BUILD SUCCESSFUL in 45s", "21 actionable tasks: 20 executed, 1 up-to-date", "tasks: 21 up-to-date=1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gradle success summary:\n%s", want, got)
		}
	}
	if strings.Contains(got, "> Task :app:task00") {
		t.Fatalf("expected per-task noise to be dropped:\n%s", got)
	}
}

func TestGradleContractSelfCapKeepsFailureTier(t *testing.T) {
	lines := []string{"> Task :app:compileJava FAILED"}
	for i := 0; i < 120; i++ {
		lines = append(lines, fmt.Sprintf("/workspace/app/src/main/java/dev/demo/File%03d.java:%d: error: incompatible types in expression number %d", i, i+10, i))
	}
	lines = append(lines, "BUILD FAILED in 51s")
	input := strings.Join(lines, "\n")

	got := gradlefilter.SummarizeGradleUnderContract(input, 40, true)
	if !strings.Contains(got, "> Task :app:compileJava FAILED") || !strings.Contains(got, "BUILD FAILED in 51s") {
		t.Fatalf("expected failure headers to survive the self-cap:\n%s", got)
	}
	rawTokens := history.EstimateTokens(input)
	if summaryTokens := history.EstimateTokens(got); summaryTokens > (rawTokens+4)/5 {
		t.Fatalf("expected self-capped summary within contract allowance, got %d of %d raw tokens", summaryTokens, rawTokens)
	}

	kind, summary, requireRaw := gradlefilter.GradleRecoveryInfoUnderContract(input, 40, true)
	if kind != "full-output" || !strings.Contains(summary, "additional lines") || !requireRaw {
		t.Fatalf("unexpected gradle recovery info: kind=%q summary=%q requireRaw=%v", kind, summary, requireRaw)
	}
}

func TestSummarizeGradleFallsBackWithoutBuildMarkers(t *testing.T) {
	got := gradlefilter.SummarizeGradle("error: something else entirely\n", 6)
	if !strings.Contains(got, "error: something else entirely") {
		t.Fatalf("expected generic fallback content, got:\n%s", got)
	}
}
