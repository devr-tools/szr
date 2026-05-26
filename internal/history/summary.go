package history

import "sort"

type summaryProfileAccumulator struct {
	stat       ProfileStat
	durations  []int64
	confidence map[string]int
}

type summaryCommandAccumulator struct {
	stat CommandStat
}

type summaryFingerprintAccumulator struct {
	stat      FingerprintStat
	durations []int64
	rawTokens int
	filtered  int
}

type summaryCommandHotspotAccumulator struct {
	stat      CommandHotspot
	failures  int
	fallbacks int
	durations []int64
	rawTokens int
	filtered  int
}

func Summarize(records []Record, limit int) Summary {
	summary := Summary{
		Profiles: make(map[string]int),
	}
	if len(records) == 0 {
		return summary
	}

	commands := map[string]*summaryCommandAccumulator{}
	profileStats := map[string]*summaryProfileAccumulator{}
	commandHotspots := map[string]*summaryCommandHotspotAccumulator{}
	fingerprintStats := map[string]*summaryFingerprintAccumulator{}
	durations := make([]int64, 0, len(records))

	for _, raw := range records {
		rec := hydrateRecord(raw)
		updateSummaryTotals(&summary, rec)
		durations = append(durations, rec.DurationMS)
		updateSummaryCommand(commands, rec)
		updateSummaryProfile(profileStats, rec)
		updateSummaryCommandHotspot(commandHotspots, rec)
		updateSummaryFingerprint(fingerprintStats, rec)
	}

	summary.AveragePct /= float64(summary.Commands)
	summary.FailureRate = percent(summary.Failures, summary.Commands)
	summary.FallbackRate = percent(summary.Fallbacks, summary.Commands)
	summary.TeeRate = percent(summary.TeeCount, summary.Commands)
	summary.DurationP50MS = percentile(durations, 50)
	summary.DurationP95MS = percentile(durations, 95)
	summary.TopCommands = summarizeTopCommands(commands, limit)
	summary.Recent = summarizeRecent(records, limit)
	summary.ProfileStats = summarizeProfiles(profileStats)
	summary.CommandHotspots = summarizeCommandHotspots(commandHotspots, limit)
	summary.FingerprintHotspots = summarizeFingerprints(fingerprintStats, limit)
	summary.BudgetSuggestions = SuggestBudgets(records, BudgetSuggestionOptions{Limit: limit})

	return summary
}

func updateSummaryTotals(summary *Summary, rec Record) {
	summary.Commands++
	summary.SavedTokens += rec.SavedTokens
	summary.RawTokens += rec.RawTokens
	summary.FilteredTokens += rec.FilteredTokens
	summary.TotalDurationMS += rec.DurationMS
	summary.AveragePct += rec.SavingsPct
	summary.RawBytesRead += rec.RawBytesRead
	summary.BytesParsed += rec.BytesParsed
	summary.BytesEmitted += rec.BytesEmitted
	if rec.ExitCode != 0 {
		summary.Failures++
	}
	if rec.FallbackUsed {
		summary.Fallbacks++
	}
	if rec.TeePath != "" {
		summary.TeeCount++
	}
	summary.Profiles[rec.Profile]++
}

func updateSummaryCommand(commands map[string]*summaryCommandAccumulator, rec Record) {
	normalizedCommand := normalizeCommand(rec.Command)
	command := commands[normalizedCommand]
	if command == nil {
		command = &summaryCommandAccumulator{stat: CommandStat{Command: normalizedCommand}}
		commands[normalizedCommand] = command
	}
	command.stat.Count++
	command.stat.AveragePct += rec.SavingsPct
	command.stat.SavedTokens += rec.SavedTokens
	command.stat.RawTokens += rec.RawTokens
	command.stat.FilteredTokens += rec.FilteredTokens
}

func updateSummaryProfile(profileStats map[string]*summaryProfileAccumulator, rec Record) {
	profile := profileStats[rec.Profile]
	if profile == nil {
		profile = &summaryProfileAccumulator{
			stat:       ProfileStat{Name: rec.Profile},
			confidence: map[string]int{},
		}
		profileStats[rec.Profile] = profile
	}
	profile.stat.Commands++
	profile.stat.AveragePct += rec.SavingsPct
	profile.stat.SavedTokens += rec.SavedTokens
	profile.stat.RawTokens += rec.RawTokens
	profile.stat.FilteredTokens += rec.FilteredTokens
	if rec.ExitCode != 0 {
		profile.stat.Failures++
	}
	if rec.FallbackUsed {
		profile.stat.Fallbacks++
	}
	if rec.TeePath != "" {
		profile.stat.TeeCount++
	}
	if rec.ProfileConfidence != "" {
		profile.confidence[rec.ProfileConfidence]++
	}
	profile.durations = append(profile.durations, rec.DurationMS)
}

func updateSummaryCommandHotspot(commandHotspots map[string]*summaryCommandHotspotAccumulator, rec Record) {
	normalizedCommand := normalizeCommand(rec.Command)
	key := normalizedCommand + "\x00" + rec.Profile
	hotspot := commandHotspots[key]
	if hotspot == nil {
		hotspot = &summaryCommandHotspotAccumulator{
			stat: CommandHotspot{
				Command: normalizedCommand,
				Profile: rec.Profile,
			},
		}
		commandHotspots[key] = hotspot
	}
	hotspot.stat.Commands++
	hotspot.stat.AveragePct += rec.SavingsPct
	if rec.ExitCode != 0 {
		hotspot.failures++
	}
	if rec.FallbackUsed {
		hotspot.fallbacks++
	}
	hotspot.durations = append(hotspot.durations, rec.DurationMS)
	hotspot.rawTokens += rec.RawTokens
	hotspot.filtered += rec.FilteredTokens
}

func updateSummaryFingerprint(fingerprints map[string]*summaryFingerprintAccumulator, rec Record) {
	fingerprint := fingerprints[rec.CommandFingerprint]
	if fingerprint == nil {
		fingerprint = &summaryFingerprintAccumulator{
			stat: FingerprintStat{
				Fingerprint: rec.CommandFingerprint,
				Command:     rec.Command,
				Profile:     rec.Profile,
			},
		}
		fingerprints[rec.CommandFingerprint] = fingerprint
	}
	fingerprint.stat.Commands++
	fingerprint.stat.AveragePct += rec.SavingsPct
	fingerprint.durations = append(fingerprint.durations, rec.DurationMS)
	fingerprint.rawTokens += rec.RawTokens
	fingerprint.filtered += rec.FilteredTokens
}

func summarizeTopCommands(commands map[string]*summaryCommandAccumulator, limit int) []CommandStat {
	list := make([]CommandStat, 0, len(commands))
	for _, command := range commands {
		command.stat.AveragePct /= float64(command.stat.Count)
		list = append(list, command.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Command < list[j].Command
		}
		return list[i].Count > list[j].Count
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

func summarizeRecent(records []Record, limit int) []Record {
	recent := append([]Record(nil), records...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if limit > 0 && len(recent) > limit {
		recent = recent[:limit]
	}
	for i := range recent {
		recent[i] = hydrateRecord(recent[i])
	}
	return recent
}

func summarizeProfiles(profileStats map[string]*summaryProfileAccumulator) []ProfileStat {
	list := make([]ProfileStat, 0, len(profileStats))
	for _, profile := range profileStats {
		profile.stat.AveragePct /= float64(profile.stat.Commands)
		profile.stat.Confidence = dominantConfidence(profile.confidence)
		profile.stat.FailureRate = percent(profile.stat.Failures, profile.stat.Commands)
		profile.stat.FallbackRate = percent(profile.stat.Fallbacks, profile.stat.Commands)
		profile.stat.TeeRate = percent(profile.stat.TeeCount, profile.stat.Commands)
		profile.stat.DurationP50MS = percentile(profile.durations, 50)
		profile.stat.DurationP95MS = percentile(profile.durations, 95)
		list = append(list, profile.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Commands == list[j].Commands {
			return list[i].Name < list[j].Name
		}
		return list[i].Commands > list[j].Commands
	})
	return list
}

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

func commandHotspotSeverity(item CommandHotspot, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 64)
	emitted := float64(filteredTokens) * volumeWeight
	fallbackPenalty := float64(rawTokens) * (item.FallbackRate / 100) * 0.35
	failurePenalty := float64(rawTokens) * (item.FailureRate / 100) * 0.20
	latencyPenalty := float64(item.DurationP95MS) * volumeWeight / 10
	efficiencyPenalty := maxFloat(0, 12-item.AveragePct) * float64(rawTokens) / 100
	return emitted + fallbackPenalty + failurePenalty + latencyPenalty + efficiencyPenalty
}

func fingerprintHotspotSeverity(item FingerprintStat, rawTokens, filteredTokens int) float64 {
	volumeWeight := scaledVolumeWeight(rawTokens, 32)
	overhead := 0.0
	if item.AveragePct < 0 {
		overhead = -item.AveragePct * float64(rawTokens) / 100 * 2
	}
	poorSavings := maxFloat(0, 10-item.AveragePct) * float64(rawTokens) / 100
	return (float64(filteredTokens) + overhead + poorSavings) * volumeWeight
}
