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

// shouldGuardSmallOutput reports whether the never-worse-than-raw guard
// applies. When raw output is already tiny, no profile should emit a
// rendering that costs more tokens than relaying raw, so the guard is
// universal (passthrough included).
func shouldGuardSmallOutput(_ Profile, _ bool) bool {
	return true
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

// shouldKeepCanonicalSmallSummary keeps a successful rendering that carries
// a canonical compact-summary marker (for example "... +N" or "dirs: "): in
// any profile such a marker signals an intentional compact summary worth
// keeping even when it is not strictly cheaper than tiny raw output.
func shouldKeepCanonicalSmallSummary(_ Profile, rendered string, rawCombined string, exitCode int) bool {
	if exitCode != 0 || rendered == rawCombined {
		return false
	}
	// The marker earns a summary modest slack over tiny raw output, but a
	// "summary" that outweighs what it summarizes by more than a quarter is
	// not one - prefer the raw output instead.
	if history.EstimateTokens(rendered)*4 > history.EstimateTokens(rawCombined)*5 {
		return false
	}
	return hasCanonicalCompactSummaryMarker(rendered)
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
