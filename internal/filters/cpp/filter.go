package cpp

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeCTest(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := []string{}
	summaries := []string{}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Test project"),
			strings.HasPrefix(trimmed, "Total Test time"),
			strings.HasPrefix(trimmed, "The following tests FAILED:"),
			strings.HasPrefix(trimmed, "Errors while running CTest"):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		case strings.Contains(trimmed, "***Failed"),
			strings.Contains(trimmed, "***Exception"),
			strings.Contains(trimmed, "Failed "),
			strings.Contains(trimmed, ".cpp:"),
			strings.Contains(trimmed, ".cc:"),
			strings.Contains(trimmed, ".c:"),
			strings.Contains(trimmed, "Assertion"),
			strings.Contains(trimmed, "error:"),
			strings.Contains(trimmed, "FAILED"):
			lines = append(lines, shared.Clip(trimmed, 160))
		}
	}

	lines = shared.UniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = shared.UniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}
	stackLines := []string{}
	rootLines := []string{}
	for _, line := range lines {
		if shared.DiagnosticAnchor(line) != "" {
			stackLines = append(stackLines, line)
			continue
		}
		rootLines = append(rootLines, line)
	}
	return shared.JoinLimitedLines(append(append(rootLines, shared.SelectUniqueAnchoredLines(stackLines, maxLines/3+1)...), summaries...), maxLines)
}

func SummarizeClangTooling(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := []string{}
	summaries := []string{}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.Contains(trimmed, ".cpp:"),
			strings.Contains(trimmed, ".cc:"),
			strings.Contains(trimmed, ".c:"),
			strings.Contains(trimmed, ".h:"),
			strings.Contains(trimmed, ".hpp:"),
			strings.Contains(trimmed, "warning:"),
			strings.Contains(trimmed, "error:"),
			strings.Contains(trimmed, "[clang-"),
			strings.Contains(trimmed, "formatting issue"),
			strings.Contains(trimmed, "cannot format"),
			strings.Contains(trimmed, "Compilation database"):
			lines = append(lines, shared.Clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "Enabled checks:"),
			strings.HasPrefix(trimmed, "Suppressed "),
			strings.HasPrefix(trimmed, "Formatting "),
			strings.HasPrefix(trimmed, "Running "),
			strings.HasPrefix(trimmed, "bear:"):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		}
	}

	lines = shared.UniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = shared.UniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}
	stackLines := []string{}
	rootLines := []string{}
	for _, line := range lines {
		if shared.DiagnosticAnchor(line) != "" {
			stackLines = append(stackLines, line)
			continue
		}
		rootLines = append(rootLines, line)
	}
	return shared.JoinLimitedLines(append(append(rootLines, shared.SelectUniqueAnchoredLines(stackLines, maxLines/3+1)...), summaries...), maxLines)
}
