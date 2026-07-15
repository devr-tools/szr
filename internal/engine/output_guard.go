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

// enforceFinalNeverWorseThanRaw is the last render pass: the emitted display
// (body plus any recovery or artifact suffix, retention repairs included)
// must never cost more tokens than relaying the raw output. Failure escapes
// may expand a render, but only within raw size — when the finished display
// would exceed it, the raw output wins on both cost and fidelity, and any
// artifact pointer becomes pure redundancy because the display omits
// nothing. Only a complete capture can stand in for the raw stream.
// Ultra-compact mode is exempt, matching the other never-worse-than-raw
// guards: the user opted into a reshaped display over token minimality.
func enforceFinalNeverWorseThanRaw(rendered string, rawCombined string, passthrough bool, captureComplete bool, ultraCompact bool, memo history.TokenMemo) string {
	if passthrough || ultraCompact || !captureComplete {
		return rendered
	}
	raw := strings.TrimSpace(rawCombined)
	if raw == "" {
		return rendered
	}
	if memo.Estimate(rendered) > memo.Estimate(raw) {
		return raw
	}
	return rendered
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
	// The byte gate runs before the token estimate so large outputs skip the
	// scan entirely; only outputs already within the byte cap get estimated.
	if len(rawCombined) > neverWorseThanRawMaxBytes {
		return false
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if rawTokens > neverWorseThanRawMaxTokens {
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
	// No summary of a complete tiny output justifies costing more than the
	// output itself: the marker only breaks the equal-size tie, never a
	// deficit - prefer the raw output whenever it is strictly cheaper.
	if history.EstimateTokens(rendered) > history.EstimateTokens(rawCombined) {
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
