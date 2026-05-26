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

func summarizeFingerprints(fingerprintStats map[string]*summaryFingerprintAccumulator, limit int) []FingerprintStat {
	type scoredFingerprint struct {
		stat     FingerprintStat
		severity float64
	}
	scored := make([]scoredFingerprint, 0, len(fingerprintStats))
	for _, fingerprint := range fingerprintStats {
		fingerprint.stat.AveragePct /= float64(fingerprint.stat.Commands)
		fingerprint.stat.DurationP50MS = percentile(fingerprint.durations, 50)
		fingerprint.stat.DurationP95MS = percentile(fingerprint.durations, 95)
		if fingerprint.stat.Commands < 2 && fingerprint.stat.AveragePct >= 0 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 16 {
			continue
		}
		if fingerprint.stat.Commands < 2 && fingerprint.rawTokens < 24 && fingerprint.stat.AveragePct > -25 {
			continue
		}
		scored = append(scored, scoredFingerprint{
			stat:     fingerprint.stat,
			severity: fingerprintHotspotSeverity(fingerprint.stat, fingerprint.rawTokens, fingerprint.filtered),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].severity == scored[j].severity {
			if scored[i].stat.AveragePct == scored[j].stat.AveragePct {
				if scored[i].stat.Commands == scored[j].stat.Commands {
					return scored[i].stat.Command < scored[j].stat.Command
				}
				return scored[i].stat.Commands > scored[j].stat.Commands
			}
			return scored[i].stat.AveragePct < scored[j].stat.AveragePct
		}
		return scored[i].severity > scored[j].severity
	})
	if limit > 0 && len(scored) > limit {
		scored = scored[:limit]
	}
	list := make([]FingerprintStat, 0, len(scored))
	for _, item := range scored {
		list = append(list, item.stat)
	}
	return list
}
