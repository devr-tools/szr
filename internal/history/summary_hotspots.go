package history

import "sort"

func summarizeCommandHotspots(commandHotspots map[string]*summaryCommandHotspotAccumulator, limit int) []CommandHotspot {
	type scoredHotspot struct {
		stat     CommandHotspot
		severity float64
	}
	scored := make([]scoredHotspot, 0, len(commandHotspots))
	for _, hotspot := range commandHotspots {
		hotspot.stat.AveragePct /= float64(hotspot.stat.Commands)
		hotspot.stat.FailureRate = percent(hotspot.failures, hotspot.stat.Commands)
		hotspot.stat.FallbackRate = percent(hotspot.fallbacks, hotspot.stat.Commands)
		hotspot.stat.DurationP50MS = percentile(hotspot.durations, 50)
		hotspot.stat.DurationP95MS = percentile(hotspot.durations, 95)
		severity := commandHotspotSeverity(hotspot.stat, hotspot.rawTokens, hotspot.filtered)
		if hotspot.stat.Commands < 2 &&
			hotspot.rawTokens < 48 &&
			hotspot.stat.AveragePct >= 0 &&
			hotspot.stat.FallbackRate == 0 &&
			hotspot.stat.FailureRate == 0 {
			continue
		}
		scored = append(scored, scoredHotspot{stat: hotspot.stat, severity: severity})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].severity == scored[j].severity {
			if scored[i].stat.Commands == scored[j].stat.Commands {
				return scored[i].stat.Command < scored[j].stat.Command
			}
			return scored[i].stat.Commands > scored[j].stat.Commands
		}
		return scored[i].severity > scored[j].severity
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	list := make([]CommandHotspot, 0, len(scored))
	for _, item := range scored {
		list = append(list, item.stat)
	}
	return list
}

// poorSavingsThresholdPct caps entry into the "poor savings fingerprints"
// table. Fingerprints averaging at or above this savings rate are healthy
// and must never be listed as poor, regardless of how much volume they
// carry. Negative averages (output grew) always qualify.
const poorSavingsThresholdPct = 40.0

func summarizeFingerprints(fingerprintStats map[string]*summaryFingerprintAccumulator, limit int) []FingerprintStat {
	type poorFingerprint struct {
		stat FingerprintStat
		// residual is the token volume still emitted after filtering.
		// Poor performers are ranked by it so the biggest remaining
		// context cost surfaces first.
		residual int
	}
	poor := make([]poorFingerprint, 0, len(fingerprintStats))
	for _, fingerprint := range fingerprintStats {
		fingerprint.stat.AveragePct /= float64(fingerprint.stat.Commands)
		fingerprint.stat.DurationP50MS = percentile(fingerprint.durations, 50)
		fingerprint.stat.DurationP95MS = percentile(fingerprint.durations, 95)
		if fingerprint.stat.AveragePct >= poorSavingsThresholdPct {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.stat.AveragePct >= 0 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 16 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 24 && fingerprint.stat.AveragePct > -25 {
			continue
		}
		poor = append(poor, poorFingerprint{
			stat:     fingerprint.stat,
			residual: fingerprint.filtered,
		})
	}
	sort.Slice(poor, func(i, j int) bool {
		if poor[i].residual == poor[j].residual {
			if poor[i].stat.AveragePct == poor[j].stat.AveragePct {
				if poor[i].stat.Commands == poor[j].stat.Commands {
					return poor[i].stat.Command < poor[j].stat.Command
				}
				return poor[i].stat.Commands > poor[j].stat.Commands
			}
			return poor[i].stat.AveragePct < poor[j].stat.AveragePct
		}
		return poor[i].residual > poor[j].residual
	})
	if limit > 0 && len(poor) > limit {
		poor = poor[:limit]
	}
	list := make([]FingerprintStat, 0, len(poor))
	for _, item := range poor {
		list = append(list, item.stat)
	}
	return list
}
