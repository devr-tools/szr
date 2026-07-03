package build

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeBuildSystem(input string, maxLines int) string {
	return SummarizeBuildSystemUnderContract(input, maxLines, false)
}

// SummarizeBuildSystemUnderContract renders the build summary; when contract
// is true the render self-caps to the predicted engine compression-contract
// allowance so failure headers and their payload lines survive downstream
// verbatim instead of gambling on the generic token capper.
func SummarizeBuildSystemUnderContract(input string, maxLines int, contract bool) string {
	return summarizeBuildSystemResult(input, maxLines, contract).Text
}

func BuildSystemRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return BuildSystemRecoveryInfoUnderContract(input, maxLines, false)
}

// BuildSystemRecoveryInfoUnderContract mirrors
// SummarizeBuildSystemUnderContract for the recovery plan.
func BuildSystemRecoveryInfoUnderContract(input string, maxLines int, contract bool) (string, string, bool) {
	result := summarizeBuildSystemResult(input, maxLines, contract)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type buildSystemSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeBuildSystemResult(input string, maxLines int, contract bool) buildSystemSummaryResult {
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
	candidates := scanBuildLines(clean).priorityLines(maxLines)
	if len(candidates) == 0 {
		return buildSystemSummaryResult{
			Text: shared.SummarizeGenericFailure(clean, maxLines),
		}
	}
	return fitBuildSummary(candidates, input, maxLines, contract)
}

func scanBuildLines(clean string) *buildScan {
	scan := &buildScan{}
	for _, line := range shared.NonEmptyLines(clean) {
		scan.ingestLine(strings.TrimSpace(line))
	}
	return scan
}

// fitBuildSummary selects the render lines within the line budget and — when
// the compression contract is armed — the predicted token allowance.
func fitBuildSummary(candidates []shared.PriorityLine, input string, maxLines int, contract bool) buildSystemSummaryResult {
	allowance := 0
	if contract {
		allowance = shared.PredictedTokenAllowance(input, maxLines)
	}
	selected, omitted := shared.FitPriorityLines(candidates, maxLines, allowance)
	return buildSystemSummaryResult{
		Text:         strings.Join(selected, "\n"),
		OmittedCount: omitted,
	}
}

// Selection tiers for build-orchestrator output: per-failure headers name
// the failing test, target, or link step and are irreducible; the payload
// lines directly under a header (the assertion text, the missing symbol)
// explain the failure and outrank every generic error line; suite-level
// counters and remaining error content follow; progress summaries close the
// render.
const (
	buildTierFailure = iota
	buildTierFailureDetail
	buildTierError
	buildTierSummary
)

// buildScan classifies build-orchestrator output line by line. detailLeft
// carries a small budget of follow-up lines after a failure header (a failed
// surefire test, a linker "Undefined symbols" block) so the assertion or
// symbol payload under the header survives — the header alone names the
// failure, the detail line explains it.
type buildScan struct {
	lines       []shared.PriorityLine
	summaries   []string
	detailLeft  int
	detailFirst bool
}

func (s *buildScan) ingestLine(trimmed string) {
	if s.detailLeft > 0 && s.ingestDetailLine(trimmed) {
		return
	}
	switch {
	case isBuildFailureDetailHeader(trimmed):
		s.appendLine(trimmed, buildFailureHeaderTier(trimmed))
		s.detailLeft = 3
		s.detailFirst = true
	case isLinkerErrorHeader(trimmed):
		s.appendLine(trimmed, buildTierFailure)
		s.detailLeft = 4
		s.detailFirst = true
	case isMavenBoilerplateLine(trimmed):
	case isBuildErrorHeaderLine(trimmed):
		s.appendLine(trimmed, buildErrorHeaderTier(trimmed))
	case isBuildErrorContentLine(trimmed):
		s.appendLine(trimmed, buildTierError)
	case isBuildProgressSummary(trimmed):
		s.summaries = append(s.summaries, shared.Clip(trimmed, 160))
	}
}

// ingestDetailLine keeps a follow-up line under an active failure header.
// Stack frames and the next tool-tagged line end the detail window. The
// first payload line is part of the irreducible failure — the header names
// the failure, this line explains it — so it shares the header's tier.
func (s *buildScan) ingestDetailLine(trimmed string) bool {
	if trimmed == "" ||
		strings.HasPrefix(trimmed, "at ") ||
		strings.HasPrefix(trimmed, "[INFO]") ||
		strings.HasPrefix(trimmed, "[ERROR]") ||
		strings.HasPrefix(trimmed, "[WARNING]") {
		s.detailLeft = 0
		return false
	}
	tier := buildTierFailureDetail
	if s.detailFirst {
		tier = buildTierFailure
		s.detailFirst = false
	}
	s.appendLine(trimmed, tier)
	s.detailLeft--
	return true
}

func (s *buildScan) appendLine(trimmed string, tier int) {
	s.lines = append(s.lines, shared.PriorityLine{Text: shared.Clip(trimmed, 160), Tier: tier})
}

// priorityLines assembles the scan's render candidates in display order:
// failure headers with their details and error content in source order (with
// anchored duplicates pruned), then progress summaries.
func (s *buildScan) priorityLines(maxLines int) []shared.PriorityLine {
	lines := dedupeBuildLines(s.lines, maxLines)
	summaries := shared.UniqueStrings(shared.FoldConsecutiveLines(s.summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return nil
	}
	out := append([]shared.PriorityLine{}, lines...)
	for _, line := range summaries {
		out = append(out, shared.PriorityLine{Text: line, Tier: buildTierSummary})
	}
	return out
}

// dedupeBuildLines drops repeated lines (keeping the first, most contextual
// occurrence) and caps anchored diagnostic lines the way the previous
// anchor-selection pass did: the same file:line reference restated with
// different prefixes carries no extra signal.
func dedupeBuildLines(lines []shared.PriorityLine, maxLines int) []shared.PriorityLine {
	seen := map[string]struct{}{}
	anchorBudget := maxLines/3 + 1
	out := make([]shared.PriorityLine, 0, len(lines))
	for _, line := range lines {
		key := line.Text
		if anchor := shared.DiagnosticAnchor(line.Text); anchor != "" && line.Tier == buildTierError {
			if anchorBudget <= 0 {
				continue
			}
			key = anchor
			anchorBudget--
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out
}

// buildFailureHeaderTier ranks surefire-style failure headers: the per-test
// header ("Class.method ... <<< FAILURE!") names the failing test and is
// irreducible, while the per-suite counter header ("Tests run: ...") merely
// aggregates it.
func buildFailureHeaderTier(trimmed string) int {
	if strings.Contains(trimmed, "Tests run:") {
		return buildTierError
	}
	return buildTierFailure
}

// buildErrorHeaderTier ranks tool-level failure headers: a build tool's own
// failure line ("make: ***", "FAILED:", "ninja: error:") names the failed
// target and is irreducible, while Maven's [ERROR]-tagged content lines are
// generic error content.
func buildErrorHeaderTier(trimmed string) int {
	if strings.HasPrefix(trimmed, "[ERROR]") || strings.HasPrefix(trimmed, "[WARNING]") {
		return buildTierError
	}
	return buildTierFailure
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
