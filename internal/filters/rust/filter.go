package rust

import (
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeCargoTest(input string, maxLines int) string {
	return summarizeCargoTestResult(input, maxLines).Text
}

func CargoTestRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeCargoTestResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

type rustSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeCargoTestResult(input string, maxLines int) rustSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return rustSummaryResult{Text: "ok"}
	}

	summaries := []string{}
	resultLine := ""
	rerunLine := ""
	failures := []string{}
	details := []string{}
	inFailureList := false
	for _, line := range lines {
		inFailureList = collectCargoTestLine(
			strings.TrimSpace(line),
			&summaries,
			&resultLine,
			&rerunLine,
			&failures,
			&details,
			inFailureList,
		)
	}

	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	failures = uniqueStrings(shared.FoldConsecutiveLines(failures))
	details = uniqueStrings(shared.FoldConsecutiveLines(details))

	return renderCargoTestSummary(input, summaries, failures, details, resultLine, rerunLine, maxLines)
}

func collectCargoTestLine(
	trimmed string,
	summaries *[]string,
	resultLine *string,
	rerunLine *string,
	failures *[]string,
	details *[]string,
	inFailureList bool,
) bool {
	switch {
	case strings.HasPrefix(trimmed, "running "):
		*summaries = append(*summaries, clip(trimmed, 160))
	case strings.HasPrefix(trimmed, "test result:"):
		*resultLine = clip(trimmed, 160)
		*summaries = append(*summaries, *resultLine)
	case strings.HasPrefix(trimmed, "error: test failed"):
		*rerunLine = clip(trimmed, 160)
		*summaries = append(*summaries, *rerunLine)
	case strings.HasPrefix(trimmed, "Finished `test`"):
		*summaries = append(*summaries, clip(trimmed, 160))
	case strings.HasPrefix(trimmed, "failures:"):
		return true
	case strings.HasPrefix(trimmed, "test ") && strings.Contains(trimmed, "FAILED"):
		*failures = append(*failures, clip(trimmed, 160))
	case isCargoDiagnosticHeader(trimmed):
		*failures = append(*failures, clip(trimmed, 160))
	case inFailureList && isCargoFailureName(trimmed):
		*failures = append(*failures, clip(trimmed, 160))
	case isCargoInterestingDetail(trimmed):
		*details = append(*details, clip(trimmed, 160))
	default:
		if trimmed == "" || isDividerLine(trimmed) {
			return inFailureList
		}
	}
	return false
}

func renderCargoTestSummary(
	input string,
	summaries []string,
	failures []string,
	details []string,
	resultLine string,
	rerunLine string,
	maxLines int,
) rustSummaryResult {
	if len(failures) == 0 && len(details) == 0 {
		return fallbackCargoTestSummary(input, summaries, resultLine, rerunLine, maxLines)
	}

	stackDetails, hints, rootDetails := splitCargoDetails(details)
	failures = pruneCargoFailureNames(failures)
	out := append([]string{}, failures...)
	out = append(out, rootDetails...)
	out = append(out, shared.SelectUniqueAnchoredLines(stackDetails, maxLines/3+1)...)
	if resultLine != "" {
		out = append(out, resultLine)
	}
	if rerunLine != "" {
		out = append(out, rerunLine)
	}
	out = append(out, hints...)
	return summarizeRustLines(out, maxLines)
}

func fallbackCargoTestSummary(input string, summaries []string, resultLine string, rerunLine string, maxLines int) rustSummaryResult {
	if resultLine != "" {
		return rustSummaryResult{Text: resultLine}
	}
	if rerunLine != "" {
		return rustSummaryResult{Text: rerunLine}
	}
	if len(summaries) > 0 {
		return summarizeRustLines(summaries, maxLines)
	}
	return rustSummaryResult{Text: shared.CompactLines(input, maxLines)}
}

func splitCargoDetails(details []string) ([]string, []string, []string) {
	stackDetails := []string{}
	hints := []string{}
	rootDetails := []string{}
	for _, line := range details {
		switch {
		case strings.HasPrefix(line, "= help:"), strings.HasPrefix(line, "= note:"), strings.HasPrefix(line, "help:"), strings.HasPrefix(line, "note:"):
			hints = append(hints, line)
		case strings.HasPrefix(line, "thread '"), strings.Contains(line, " panicked at "):
			rootDetails = append(rootDetails, line)
		case shared.DiagnosticAnchor(line) != "" || strings.HasPrefix(line, "--> "):
			stackDetails = append(stackDetails, line)
		default:
			rootDetails = append(rootDetails, line)
		}
	}
	return stackDetails, hints, rootDetails
}

func SummarizeCargoBuild(input string, maxLines int) string {
	return summarizeCargoBuildResult(input, maxLines).Text
}

func CargoBuildRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeCargoBuildResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

func summarizeCargoBuildResult(input string, maxLines int) rustSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return rustSummaryResult{Text: "ok"}
	}

	summaries := []string{}
	diagnostics := []string{}
	details := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "error: could not compile"),
			strings.HasPrefix(trimmed, "Finished "),
			strings.HasPrefix(trimmed, "Checking "),
			strings.HasPrefix(trimmed, "Compiling "):
			summaries = append(summaries, clip(trimmed, 160))
		case isCargoDiagnosticHeader(trimmed):
			diagnostics = append(diagnostics, clip(trimmed, 160))
		case isCargoInterestingDetail(trimmed):
			details = append(details, clip(trimmed, 160))
		}
	}

	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	diagnostics = uniqueStrings(shared.FoldConsecutiveLines(diagnostics))
	details = uniqueStrings(shared.FoldConsecutiveLines(details))

	if len(diagnostics) == 0 && len(details) == 0 {
		if len(summaries) > 0 {
			return summarizeRustLines(summaries, maxLines)
		}
		return rustSummaryResult{Text: shared.CompactLines(input, maxLines)}
	}

	stackDetails := []string{}
	hints := []string{}
	rootDetails := []string{}
	for _, line := range details {
		switch {
		case strings.HasPrefix(line, "= help:"), strings.HasPrefix(line, "= note:"), strings.HasPrefix(line, "help:"), strings.HasPrefix(line, "note:"):
			hints = append(hints, line)
		case shared.DiagnosticAnchor(line) != "" || strings.HasPrefix(line, "--> "):
			stackDetails = append(stackDetails, line)
		default:
			rootDetails = append(rootDetails, line)
		}
	}

	out := append([]string{}, diagnostics...)
	out = append(out, rootDetails...)
	out = append(out, shared.SelectUniqueAnchoredLines(stackDetails, maxLines/3+1)...)
	out = append(out, hints...)
	out = append(out, summaries...)
	return summarizeRustLines(out, maxLines)
}

func isCargoDiagnosticHeader(line string) bool {
	switch {
	case strings.HasPrefix(line, "error["),
		strings.HasPrefix(line, "warning:"),
		strings.HasPrefix(line, "error:"),
		strings.HasPrefix(line, "clippy::"):
		return true
	default:
		return false
	}
}

func isCargoFailureName(line string) bool {
	if line == "" {
		return false
	}
	if strings.Contains(line, "FAILED") || strings.Contains(line, "error:") {
		return false
	}
	return !strings.Contains(line, " ")
}

func isCargoInterestingDetail(line string) bool {
	switch {
	case line == "":
		return false
	case strings.HasPrefix(line, "--> "),
		strings.HasPrefix(line, "= help:"),
		strings.HasPrefix(line, "= note:"),
		strings.HasPrefix(line, "help:"),
		strings.HasPrefix(line, "note:"),
		strings.HasPrefix(line, "thread '"),
		strings.Contains(line, " panicked at "),
		strings.Contains(line, "Assertion"),
		strings.Contains(line, "assertion `"),
		strings.Contains(line, ".rs:"),
		strings.Contains(line, "unresolved import"),
		strings.Contains(line, "mismatched types"),
		strings.Contains(line, "unused "),
		strings.Contains(line, "cannot find "),
		strings.Contains(line, "failed to "),
		strings.Contains(line, "expected "),
		strings.Contains(line, "found "):
		return true
	default:
		return false
	}
}

func StripANSI(input string) string {
	return shared.StripANSI(input)
}

func clip(input string, max int) string {
	return shared.Clip(input, max)
}

func uniqueStrings(values []string) []string {
	return shared.UniqueStrings(values)
}

func nonEmptyLines(input string) []string {
	return shared.NonEmptyLines(input)
}

func joinLimitedLines(lines []string, maxLines int) string {
	return shared.JoinLimitedLines(lines, maxLines)
}

func summarizeRustLines(lines []string, maxLines int) rustSummaryResult {
	result := rustSummaryResult{Text: shared.JoinLimitedLines(lines, maxLines)}
	if len(lines) > maxLines {
		result.OmittedCount = len(lines) - maxLines
	}
	return result
}

func isDividerLine(line string) bool {
	return strings.Trim(line, "=-_ ") == ""
}

func pruneCargoFailureNames(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}
	explicit := map[string]struct{}{}
	for _, line := range lines {
		if strings.HasPrefix(line, "test ") && strings.Contains(line, " ... FAILED") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "test "), " ... FAILED"))
			explicit[name] = struct{}{}
		}
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := explicit[line]; ok {
			continue
		}
		out = append(out, line)
	}
	return out
}
