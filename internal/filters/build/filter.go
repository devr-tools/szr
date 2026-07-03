package build

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeBuildSystem(input string, maxLines int) string {
	return summarizeBuildSystemResult(input, maxLines).Text
}

func BuildSystemRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeBuildSystemResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type buildSystemSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeBuildSystemResult(input string, maxLines int) buildSystemSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	if isTerraformOutput(clean) {
		return summarizeTerraformResult(clean, maxLines)
	}
	if isBuildKitOutput(clean) {
		return summarizeBuildKitResult(clean, maxLines)
	}
	scan := &buildScan{}
	for _, line := range shared.NonEmptyLines(clean) {
		scan.ingestLine(strings.TrimSpace(line))
	}

	lines := shared.UniqueStrings(shared.FoldConsecutiveLines(scan.lines))
	summaries := shared.UniqueStrings(shared.FoldConsecutiveLines(scan.summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return buildSystemSummaryResult{
			Text: shared.SummarizeGenericFailure(clean, maxLines),
		}
	}

	anchors := []string{}
	other := []string{}
	for _, line := range lines {
		if shared.DiagnosticAnchor(line) != "" {
			anchors = append(anchors, line)
			continue
		}
		other = append(other, line)
	}

	out := append([]string{}, other...)
	out = append(out, shared.SelectUniqueAnchoredLines(anchors, maxLines/3+1)...)
	out = append(out, summaries...)
	result := buildSystemSummaryResult{
		Text: shared.JoinLimitedLines(out, maxLines),
	}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}

// buildScan classifies build-orchestrator output line by line. detailLeft
// carries a small budget of follow-up lines after a failure header (a failed
// surefire test, a linker "Undefined symbols" block) so the assertion or
// symbol payload under the header survives — the header alone names the
// failure, the detail line explains it.
type buildScan struct {
	lines      []string
	summaries  []string
	detailLeft int
}

func (s *buildScan) ingestLine(trimmed string) {
	if s.detailLeft > 0 && s.ingestDetailLine(trimmed) {
		return
	}
	switch {
	case isBuildFailureDetailHeader(trimmed):
		s.lines = append(s.lines, shared.Clip(trimmed, 160))
		s.detailLeft = 3
	case isLinkerErrorHeader(trimmed):
		s.lines = append(s.lines, shared.Clip(trimmed, 160))
		s.detailLeft = 4
	case isMavenBoilerplateLine(trimmed):
	case isBuildErrorHeaderLine(trimmed), isBuildErrorContentLine(trimmed):
		s.lines = append(s.lines, shared.Clip(trimmed, 160))
	case isBuildProgressSummary(trimmed):
		s.summaries = append(s.summaries, shared.Clip(trimmed, 160))
	}
}

// ingestDetailLine keeps a follow-up line under an active failure header.
// Stack frames and the next tool-tagged line end the detail window.
func (s *buildScan) ingestDetailLine(trimmed string) bool {
	if trimmed == "" ||
		strings.HasPrefix(trimmed, "at ") ||
		strings.HasPrefix(trimmed, "[INFO]") ||
		strings.HasPrefix(trimmed, "[ERROR]") ||
		strings.HasPrefix(trimmed, "[WARNING]") {
		s.detailLeft = 0
		return false
	}
	s.lines = append(s.lines, shared.Clip(trimmed, 160))
	s.detailLeft--
	return true
}

func isBuildErrorHeaderLine(trimmed string) bool {
	for _, prefix := range []string{
		"make: ***", "just: error:", "task: Failed to run task",
		"FAILED:", "ERROR:", "[ERROR]", "[WARNING]",
		"ninja: error:", "ninja: build stopped:", "CMake Error", "Target //",
		"Build did NOT complete successfully",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func isBuildErrorContentLine(trimmed string) bool {
	for _, needle := range []string{
		"No rule to make target", "recipe for target", "error generated",
		"FAILED in", "failed to solve", "failed with exit code",
		"failed:", "error:", ".cc:", ".cpp:", ".c:", ".h:", ".hpp:",
		"undefined reference", "symbol(s) not found",
	} {
		if strings.Contains(trimmed, needle) {
			return true
		}
	}
	return false
}

// isBuildFailureDetailHeader recognizes per-test failure headers (Maven
// surefire style) whose payload lives on the following unprefixed lines.
func isBuildFailureDetailHeader(trimmed string) bool {
	return strings.HasPrefix(trimmed, "[ERROR]") &&
		(strings.Contains(trimmed, "<<< FAILURE!") || strings.Contains(trimmed, "<<< ERROR!"))
}

// isLinkerErrorHeader recognizes linker failures whose named symbols follow
// on indented continuation lines.
func isLinkerErrorHeader(trimmed string) bool {
	return strings.Contains(trimmed, "Undefined symbols") ||
		strings.Contains(trimmed, "duplicate symbol")
}

// isMavenBoilerplateLine drops the advisory [ERROR] tail Maven prints after
// the real failures; it would otherwise crowd assertion details out of the
// line budget.
func isMavenBoilerplateLine(trimmed string) bool {
	rest, ok := strings.CutPrefix(trimmed, "[ERROR]")
	if !ok {
		return false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return true
	}
	for _, prefix := range []string{
		"-> [Help", "[Help", "Re-run Maven", "To see the full stack trace",
		"For more information about", "See ",
	} {
		if strings.HasPrefix(rest, prefix) {
			return true
		}
	}
	return false
}

func isBuildProgressSummary(trimmed string) bool {
	for _, prefix := range []string{
		"Built target ", "[", "Scanning dependencies",
		"-- Configuring", "-- Generating", "INFO: Analyzed",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
