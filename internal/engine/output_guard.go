package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const (
	neverWorseThanRawMaxBytes  = 384
	neverWorseThanRawMaxTokens = 96
)

func preferRawSmallOutput(rendered string, rawCombined string) string {
	return preferRawSmallOutputForProfile(Profile{}, rendered, rawCombined, 0)
}

func preferRawSmallOutputForProfile(profile Profile, rendered string, rawCombined string, exitCode int) string {
	if !shouldPreferRawSmallOutput(profile, rendered, rawCombined, exitCode) {
		return rendered
	}
	return strings.TrimSpace(rawCombined)
}

func shouldGuardSmallOutput(profile Profile, passthrough bool) bool {
	if passthrough {
		return true
	}
	switch profile.Name {
	case "generic-summary",
		"grep",
		"path-find",
		"ripgrep",
		"ripgrep-files",
		"ripgrep-files-with-matches":
		return true
	default:
		return false
	}
}

func shouldPreferRawSmallOutput(profile Profile, rendered string, rawCombined string, exitCode int) bool {
	rawCombined = strings.TrimSpace(rawCombined)
	rendered = strings.TrimSpace(rendered)
	if rawCombined == "" || rendered == "" {
		return false
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if len(rawCombined) > neverWorseThanRawMaxBytes || rawTokens > neverWorseThanRawMaxTokens {
		return false
	}
	if shouldKeepCanonicalSmallSummary(profile, rendered, rawCombined, exitCode) {
		return false
	}
	renderedTokens := history.EstimateTokens(rendered)
	if renderedTokens > rawTokens {
		return true
	}
	return renderedTokens == rawTokens && len(rendered) >= len(rawCombined)
}

func shouldKeepCanonicalSmallSummary(profile Profile, rendered string, rawCombined string, exitCode int) bool {
	if exitCode != 0 || rendered == rawCombined {
		return false
	}
	switch profile.Name {
	case "generic-summary",
		"grep",
		"path-find",
		"ripgrep",
		"ripgrep-files",
		"ripgrep-files-with-matches":
		return hasCanonicalCompactSummaryMarker(rendered)
	default:
		return false
	}
}

func hasCanonicalCompactSummaryMarker(rendered string) bool {
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return false
	}
	lines := strings.Split(rendered, "\n")
	if hasCanonicalCompactSummaryHeadline(lines[0]) {
		return true
	}
	return hasCanonicalCompactSummaryTail(lines[1:])
}

func hasCanonicalCompactSummaryHeadline(first string) bool {
	return first == "no matches" ||
		strings.Contains(first, " matches across ") ||
		strings.Contains(first, " matches | ") ||
		(strings.Contains(first, "F ") && strings.Contains(first, "D | "))
}

func hasCanonicalCompactSummaryTail(lines []string) bool {
	for _, line := range lines {
		if isCanonicalCompactSummaryTailLine(line) {
			return true
		}
	}
	return false
}

func isCanonicalCompactSummaryTailLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "dirs: "):
		return true
	case strings.HasPrefix(line, "examples: "):
		return true
	case strings.HasPrefix(line, "suppressed noisy paths: "):
		return true
	case strings.HasPrefix(line, "... +"):
		return true
	default:
		return false
	}
}
