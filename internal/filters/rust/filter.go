package rust

import (
	"strings"

	shared "szr/internal/filters"
)

func SummarizeCargoTest(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return "ok"
	}

	summaries := []string{}
	failures := []string{}
	details := []string{}
	inFailureList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "running "):
			summaries = append(summaries, clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "test result:"):
			summaries = append(summaries, clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "error: test failed"):
			summaries = append(summaries, clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "Finished `test`"):
			summaries = append(summaries, clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "failures:"):
			inFailureList = true
		case strings.HasPrefix(trimmed, "test ") && strings.Contains(trimmed, "FAILED"):
			failures = append(failures, clip(trimmed, 160))
		case isCargoDiagnosticHeader(trimmed):
			failures = append(failures, clip(trimmed, 160))
			inFailureList = false
		case inFailureList && isCargoFailureName(trimmed):
			failures = append(failures, clip(trimmed, 160))
		case isCargoInterestingDetail(trimmed):
			details = append(details, clip(trimmed, 160))
			inFailureList = false
		default:
			if trimmed == "" || isDividerLine(trimmed) {
				continue
			}
			inFailureList = false
		}
	}

	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	failures = uniqueStrings(shared.FoldConsecutiveLines(failures))
	details = uniqueStrings(shared.FoldConsecutiveLines(details))

	if len(failures) == 0 && len(details) == 0 {
		if len(summaries) > 0 {
			return joinLimitedLines(summaries, maxLines)
		}
		return shared.CompactLines(input, maxLines)
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

	out := append([]string{}, summaries...)
	out = append(out, failures...)
	out = append(out, rootDetails...)
	out = append(out, shared.SelectUniqueAnchoredLines(stackDetails, maxLines/3+1)...)
	out = append(out, hints...)
	return joinLimitedLines(out, maxLines)
}

func SummarizeCargoBuild(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return "ok"
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
			return joinLimitedLines(summaries, maxLines)
		}
		return shared.CompactLines(input, maxLines)
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
	return joinLimitedLines(out, maxLines)
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

func isDividerLine(line string) bool {
	return strings.Trim(line, "=-_ ") == ""
}
