package workflows

import (
	"fmt"
	"strings"
)

const (
	hotspotSignalRepeatedPassthrough = "repeated-passthrough"
	hotspotSignalLowSavings          = "low-savings"
	hotspotSignalFallbackHeavy       = "fallback-heavy"
	hotspotSignalTeeHeavy            = "tee-heavy"
)

func annotateHotspotSignals(item *HotspotStat) {
	item.Signals = hotspotSignals(*item)
	item.CoverageScore = hotspotCoverageScore(*item)
}

func hotspotSignals(item HotspotStat) []string {
	signals := make([]string, 0, 4)
	if isGenericHotspot(item) && item.Samples >= 3 {
		signals = append(signals, hotspotSignalRepeatedPassthrough)
	}
	if hotspotHasLowSavingsGap(item) {
		signals = append(signals, hotspotSignalLowSavings)
	}
	if item.Fallbacks >= 2 && item.FallbackRate >= 50 {
		signals = append(signals, hotspotSignalFallbackHeavy)
	}
	if item.TeeCount >= 2 && item.TeeRate >= 50 {
		signals = append(signals, hotspotSignalTeeHeavy)
	}
	return signals
}

func hotspotCoverageScore(item HotspotStat) int {
	score := 0
	for _, signal := range item.Signals {
		switch signal {
		case hotspotSignalRepeatedPassthrough:
			score += 2
		case hotspotSignalLowSavings:
			score += 3
		case hotspotSignalFallbackHeavy:
			score += 3
		case hotspotSignalTeeHeavy:
			score += 2
		}
	}
	if isGenericHotspot(item) && item.Samples >= 2 {
		score++
	}
	if item.Samples > 2 {
		score += minInt(item.Samples-2, 4)
	}
	return score
}

func hotspotHasLowSavingsGap(item HotspotStat) bool {
	if item.Samples < 2 {
		return false
	}
	avgRaw := item.RawTokens / maxInt(item.Samples, 1)
	avgFiltered := item.FilteredTokens / maxInt(item.Samples, 1)
	if avgRaw < 72 && avgFiltered < 48 {
		return false
	}
	target := 20.0
	switch {
	case avgRaw >= 512 || avgFiltered >= 384:
		target = 30.0
	case avgRaw >= 192 || avgFiltered >= 144:
		target = 24.0
	}
	return item.AveragePct < target
}

func hotspotSignalsSummary(item HotspotStat) string {
	if len(item.Signals) == 0 {
		return fmt.Sprintf("history repeats across %d runs", item.Samples)
	}

	parts := make([]string, 0, len(item.Signals))
	for _, signal := range item.Signals {
		switch signal {
		case hotspotSignalRepeatedPassthrough:
			parts = append(parts, "repeated passthrough")
		case hotspotSignalLowSavings:
			parts = append(parts, "low savings")
		case hotspotSignalFallbackHeavy:
			parts = append(parts, "fallback-heavy runs")
		case hotspotSignalTeeHeavy:
			parts = append(parts, "tee-heavy failures")
		}
	}

	return strings.Join(parts, ", ") + fmt.Sprintf(" across %d runs", item.Samples)
}

func hotspotSignalList(item HotspotStat) string {
	if len(item.Signals) == 0 {
		return "-"
	}
	return strings.Join(item.Signals, ",")
}

func hotspotPriorityBoost(item HotspotStat, ceiling int) int {
	if item.CoverageScore <= 0 {
		return 0
	}
	if item.CoverageScore > ceiling {
		return ceiling
	}
	return item.CoverageScore
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
