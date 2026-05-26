package history

func commandHotspotSeverity(item CommandHotspot, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 64)
	emitted := float64(filteredTokens) * volumeWeight * 0.85
	fallbackPenalty := float64(rawTokens) * (item.FallbackRate / 100) * 0.25
	if item.FallbackRate >= 50 && hasHotspotSavingsOpportunity(rawTokens, filteredTokens) {
		fallbackPenalty += float64(maxInt(rawTokens, filteredTokens)) * (item.FallbackRate / 100) * 0.20
	}
	failurePenalty := float64(rawTokens) * (item.FailureRate / 100) * 0.20
	latencyPenalty := maxFloat(0, float64(item.DurationP95MS)-75) * volumeWeight / 6
	efficiencyPenalty := hotspotSavingsPenalty(item.AveragePct, rawTokens, filteredTokens)
	repetitionWeight := 1 + scaledVolumeWeight(item.Commands*16, 24)
	return (emitted + fallbackPenalty + failurePenalty + latencyPenalty + efficiencyPenalty) * repetitionWeight
}

func fingerprintHotspotSeverity(item FingerprintStat, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 32)
	overhead := 0.0
	if item.AveragePct < 0 {
		overhead = -item.AveragePct * float64(rawTokens) / 100 * 2
		if isTinyHotspotOverhead(rawTokens, filteredTokens, item.AveragePct) {
			overhead *= 0.2
		}
	}
	poorSavings := hotspotSavingsPenalty(item.AveragePct, rawTokens, filteredTokens)
	return (float64(filteredTokens) + overhead + poorSavings) * volumeWeight
}

func hasHotspotSavingsOpportunity(rawTokens, filteredTokens int) bool {
	return rawTokens >= 96 || filteredTokens >= 72
}

func isTinyHotspotOverhead(rawTokens, filteredTokens int, averagePct float64) bool {
	return rawTokens < 32 && filteredTokens < 32 && averagePct >= -20
}

func hotspotSavingsPenalty(averagePct float64, rawTokens, filteredTokens int) float64 {
	if !hasHotspotSavingsOpportunity(rawTokens, filteredTokens) {
		if isTinyHotspotOverhead(rawTokens, filteredTokens, averagePct) {
			return 0
		}
		return maxFloat(0, -averagePct) * float64(rawTokens) / 200
	}

	target := 8.0
	if rawTokens >= 256 || filteredTokens >= 192 {
		target = 12.0
	}
	if rawTokens >= 1024 || filteredTokens >= 768 {
		target = 18.0
	}
	return maxFloat(0, target-averagePct) * float64(maxInt(rawTokens, filteredTokens)) / 110
}
