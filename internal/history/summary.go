package history

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
		if rec.Passthrough {
			// Intentionally-raw runs stay in the global tallies above but
			// must not skew savings analysis or improvement suggestions.
			continue
		}
		updateSummaryCommand(commands, rec)
		updateSummaryProfile(profileStats, rec)
		if rec.RawTokens > 0 {
			// Zero-output runs carry no savings signal; keep them out of
			// improvement hotspots so recommendations target real volume.
			updateSummaryCommandHotspot(commandHotspots, rec)
			updateSummaryFingerprint(fingerprintStats, rec)
		}
	}

	if savingsSamples := summary.Commands - summary.PassthroughCommands; savingsSamples > 0 {
		summary.AveragePct /= float64(savingsSamples)
	}
	summary.FilteredSavingsPct = percent(summary.SavedTokens, summary.RawTokens-summary.PassthroughTokens)
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
	if rec.Passthrough {
		summary.PassthroughCommands++
		summary.PassthroughTokens += rec.RawTokens
	} else {
		summary.AveragePct += rec.SavingsPct
	}
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
