package php

import (
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizePHP(input string, maxLines int) string {
	return summarizePHPResult(input, maxLines).Text
}

func PHPRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizePHPResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

type phpSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizePHPResult(input string, maxLines int) phpSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := []string{}
	summaries := []string{}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(trimmed, "Composer could not"),
			strings.HasPrefix(trimmed, "Loading composer repositories"),
			strings.HasPrefix(trimmed, "Installing dependencies from lock file"),
			strings.HasPrefix(trimmed, "Package operations:"),
			strings.HasPrefix(trimmed, "There was "),
			strings.HasPrefix(trimmed, "There were "),
			strings.HasPrefix(trimmed, "FAILURES!"),
			strings.HasPrefix(trimmed, "ERRORS!"),
			strings.HasPrefix(trimmed, "PHPUnit "),
			strings.HasPrefix(trimmed, "Pest "),
			strings.HasPrefix(trimmed, "Problem "),
			strings.HasPrefix(trimmed, "Script "),
			strings.Contains(trimmed, "Your requirements could not be resolved"):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		case strings.Contains(lower, "fatal error"),
			strings.Contains(lower, "parse error"),
			strings.Contains(lower, "uncaught"),
			strings.Contains(trimmed, "Failed asserting that"),
			strings.Contains(trimmed, "Tests:"),
			strings.Contains(trimmed, "Failures:"),
			strings.Contains(trimmed, "Errors:"),
			strings.Contains(trimmed, "SQLSTATE["),
			strings.Contains(trimmed, "Call to undefined"),
			strings.Contains(trimmed, "Undefined "),
			strings.Contains(trimmed, "Class ") && strings.Contains(trimmed, "not found"),
			strings.Contains(trimmed, "Found ") && strings.Contains(lower, "error"),
			strings.Contains(trimmed, ".php:"),
			strings.Contains(trimmed, ".phtml:"),
			strings.HasPrefix(trimmed, "Error:"),
			strings.HasPrefix(trimmed, "ERROR:"),
			strings.HasPrefix(trimmed, "Line "):
			lines = append(lines, shared.Clip(trimmed, 160))
		}
	}

	lines = shared.UniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = shared.UniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return phpSummaryResult{Text: shared.SummarizeGenericFailure(clean, maxLines)}
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
	result := phpSummaryResult{Text: shared.JoinLimitedLines(out, maxLines)}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}
