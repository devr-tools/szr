package gradle

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeGradle(input string, maxLines int) string {
	return SummarizeGradleUnderContract(input, maxLines, false)
}

// SummarizeGradleUnderContract renders the gradle summary; when contract is
// true the render self-caps to the predicted engine compression-contract
// allowance so failed task headers and their diagnostics survive downstream
// verbatim instead of gambling on the generic token capper.
func SummarizeGradleUnderContract(input string, maxLines int, contract bool) string {
	return summarizeGradleResult(input, maxLines, contract).Text
}

func GradleRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return GradleRecoveryInfoUnderContract(input, maxLines, false)
}

// GradleRecoveryInfoUnderContract mirrors SummarizeGradleUnderContract for
// the recovery plan.
func GradleRecoveryInfoUnderContract(input string, maxLines int, contract bool) (string, string, bool) {
	result := summarizeGradleResult(input, maxLines, contract)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type gradleSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeGradleResult(input string, maxLines int, contract bool) gradleSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	scan := scanGradleLines(clean)
	if scan.succeeded() {
		return gradleSummaryResult{Text: strings.Join(scan.successLines(), "\n")}
	}
	candidates := scan.priorityLines()
	if len(candidates) == 0 {
		return gradleSummaryResult{Text: shared.SummarizeGenericFailure(clean, maxLines)}
	}
	allowance := 0
	if contract {
		allowance = shared.PredictedTokenAllowance(input, maxLines)
	}
	selected, omitted := shared.FitPriorityLines(candidates, maxLines, allowance)
	return gradleSummaryResult{Text: strings.Join(selected, "\n"), OmittedCount: omitted}
}

func scanGradleLines(clean string) *gradleScan {
	scan := &gradleScan{}
	for _, line := range shared.NonEmptyLines(clean) {
		scan.ingestLine(strings.TrimSpace(line))
	}
	return scan
}

// Selection tiers for gradle output: failed task headers and failing test
// names are irreducible; compiler diagnostics with file:line anchors and the
// "What went wrong" payload explain them; remaining error content follows;
// the actionable-tasks counter closes the render.
const (
	gradleTierFailure = iota
	gradleTierDetail
	gradleTierError
	gradleTierSummary
)

// gradleScan classifies gradle console output line by line. detailLeft
// carries a small budget of follow-up lines after a failure header (a failing
// test's assertion, the "What went wrong" explanation) so the payload under
// the header survives.
type gradleScan struct {
	lines           []shared.PriorityLine
	taskCount       int
	upToDate        int
	fromCache       int
	buildSuccessful string
	actionable      string
	buildFailed     bool
	detailLeft      int
}

func (s *gradleScan) ingestLine(trimmed string) {
	if s.detailLeft > 0 && s.ingestDetailLine(trimmed) {
		return
	}
	if s.ingestFailureSignal(trimmed) {
		return
	}
	switch {
	case strings.HasPrefix(trimmed, "BUILD SUCCESSFUL"):
		s.buildSuccessful = trimmed
	case strings.Contains(trimmed, "tests completed,"):
		s.appendLine(trimmed, gradleTierDetail)
	case strings.Contains(trimmed, "actionable task"):
		s.actionable = trimmed
	case isGradleNoiseLine(trimmed):
	case isGradleErrorContentLine(trimmed):
		s.appendLine(trimmed, gradleTierError)
	}
}

// ingestFailureSignal handles the failure-bearing line shapes: task stream
// lines, build failure headers, the went-wrong marker, per-test failures, and
// compiler diagnostics.
func (s *gradleScan) ingestFailureSignal(trimmed string) bool {
	switch {
	case strings.HasPrefix(trimmed, "> Task "):
		s.ingestTaskLine(trimmed)
	case strings.HasPrefix(trimmed, "BUILD FAILED"), strings.HasPrefix(trimmed, "FAILURE:"):
		s.buildFailed = true
		s.appendLine(trimmed, gradleTierFailure)
	case trimmed == "* What went wrong:":
		s.detailLeft = 3
	case isGradleTestFailureLine(trimmed):
		s.buildFailed = true
		s.appendLine(trimmed, gradleTierFailure)
		s.detailLeft = 2
	case isGradleCompileErrorLine(trimmed):
		s.appendLine(trimmed, gradleTierDetail)
	default:
		return false
	}
	return true
}

// ingestDetailLine keeps a follow-up line under an active failure header. The
// next section marker or task line ends the detail window.
func (s *gradleScan) ingestDetailLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "> Task ") {
		s.detailLeft = 0
		return false
	}
	s.appendLine(trimmed, gradleTierDetail)
	s.detailLeft--
	return true
}

func (s *gradleScan) ingestTaskLine(trimmed string) {
	s.taskCount++
	switch {
	case strings.HasSuffix(trimmed, " FAILED"):
		s.buildFailed = true
		s.appendLine(trimmed, gradleTierFailure)
	case strings.HasSuffix(trimmed, " UP-TO-DATE"):
		s.upToDate++
	case strings.HasSuffix(trimmed, " FROM-CACHE"):
		s.fromCache++
	}
}

func (s *gradleScan) appendLine(trimmed string, tier int) {
	s.lines = append(s.lines, shared.PriorityLine{Text: shared.Clip(trimmed, 160), Tier: tier})
}

func (s *gradleScan) succeeded() bool {
	return s.buildSuccessful != "" && !s.buildFailed
}

func (s *gradleScan) successLines() []string {
	out := []string{s.buildSuccessful}
	if s.actionable != "" {
		out = append(out, s.actionable)
	}
	if s.taskCount > 0 {
		out = append(out, gradleTaskCounts(s.taskCount, s.upToDate, s.fromCache))
	}
	return out
}

func gradleTaskCounts(total, upToDate, fromCache int) string {
	line := fmt.Sprintf("tasks: %d", total)
	if upToDate > 0 {
		line += fmt.Sprintf(" up-to-date=%d", upToDate)
	}
	if fromCache > 0 {
		line += fmt.Sprintf(" from-cache=%d", fromCache)
	}
	return line
}

// priorityLines assembles the scan's render candidates in source order with
// duplicates pruned (gradle restates failed task headers after the task
// stream), then the actionable-tasks counter.
func (s *gradleScan) priorityLines() []shared.PriorityLine {
	seen := map[string]struct{}{}
	out := make([]shared.PriorityLine, 0, len(s.lines))
	for _, line := range s.lines {
		if _, dup := seen[line.Text]; dup {
			continue
		}
		seen[line.Text] = struct{}{}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil
	}
	if s.actionable != "" {
		out = append(out, shared.PriorityLine{Text: s.actionable, Tier: gradleTierSummary})
	}
	return out
}

// isGradleTestFailureLine recognizes per-test failure lines from gradle's
// test task ("OrderServiceTest > refundsCancelledOrder FAILED").
func isGradleTestFailureLine(trimmed string) bool {
	return strings.HasSuffix(trimmed, " FAILED") && strings.Contains(trimmed, " > ")
}

// isGradleCompileErrorLine recognizes javac ("File.java:42: error: ...") and
// kotlinc ("e: file:...") diagnostics that carry file:line anchors.
func isGradleCompileErrorLine(trimmed string) bool {
	return strings.Contains(trimmed, ": error:") || strings.HasPrefix(trimmed, "e: ")
}

func isGradleErrorContentLine(trimmed string) bool {
	for _, needle := range []string{
		"error:", "warning:", "Caused by:", "Could not resolve", "Could not determine",
		"Could not find", "Execution failed for task",
	} {
		if strings.Contains(trimmed, needle) {
			return true
		}
	}
	return strings.HasPrefix(trimmed, "w: ")
}

// isGradleNoiseLine drops daemon chatter, progress bars, download lines, and
// the advisory tail gradle prints after the real failure.
func isGradleNoiseLine(trimmed string) bool {
	for _, prefix := range []string{
		"<", "Download ", "Downloading ", "Starting a Gradle Daemon",
		"Deprecated Gradle features", "You can use '--warning-mode",
		"> Run with --", "> Get more help", "See https://docs.gradle.org",
		"To honour the JVM settings", "Daemon will be stopped",
		"Configuration cache", "Watched directory hierarchies",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
