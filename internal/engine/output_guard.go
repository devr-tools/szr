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
	if !shouldPreferRawSmallOutput(rendered, rawCombined) {
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

func shouldPreferRawSmallOutput(rendered string, rawCombined string) bool {
	rawCombined = strings.TrimSpace(rawCombined)
	rendered = strings.TrimSpace(rendered)
	if rawCombined == "" || rendered == "" {
		return false
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if len(rawCombined) > neverWorseThanRawMaxBytes || rawTokens > neverWorseThanRawMaxTokens {
		return false
	}
	renderedTokens := history.EstimateTokens(rendered)
	if renderedTokens > rawTokens {
		return true
	}
	return renderedTokens == rawTokens && len(rendered) >= len(rawCombined)
}
