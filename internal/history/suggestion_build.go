package history

import "sort"

func SuggestBudgets(records []Record, opts BudgetSuggestionOptions) []BudgetSuggestion {
	opts = normalizeBudgetSuggestionOptions(opts)
	if len(records) == 0 {
		return nil
	}

	recent := budgetSuggestionRecentRecords(records, opts.Lookback)
	stats := accumulateBudgetSuggestions(recent)
	candidates := buildBudgetSuggestionCandidates(stats, opts.MinSamples)
	return limitBudgetSuggestions(candidates, opts.Limit)
}

func budgetSuggestionRecentRecords(records []Record, lookback int) []Record {
	recent := append([]Record(nil), records...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Timestamp.After(recent[j].Timestamp)
	})
	if lookback > 0 && len(recent) > lookback {
		recent = recent[:lookback]
	}
	return recent
}

func accumulateBudgetSuggestions(records []Record) map[string]*budgetSuggestionAccumulator {
	stats := map[string]*budgetSuggestionAccumulator{}
	for _, raw := range records {
		rec := hydrateRecord(raw)
		if rec.CommandFingerprint == "" || rec.Passthrough {
			continue
		}
		acc := stats[rec.CommandFingerprint]
		if acc == nil {
			acc = &budgetSuggestionAccumulator{
				fingerprint:   rec.CommandFingerprint,
				command:       rec.Command,
				lastSeen:      rec.Timestamp,
				firstSeen:     rec.Timestamp,
				profileCounts: map[string]int{},
			}
			stats[rec.CommandFingerprint] = acc
		}
		updateBudgetSuggestionAccumulator(acc, rec)
	}
	return stats
}

func updateBudgetSuggestionAccumulator(acc *budgetSuggestionAccumulator, rec Record) {
	if rec.Timestamp.After(acc.lastSeen) {
		acc.lastSeen = rec.Timestamp
		acc.command = rec.Command
	}
	if rec.Timestamp.Before(acc.firstSeen) {
		acc.firstSeen = rec.Timestamp
	}
	acc.profileCounts[rec.Profile]++
	acc.rawTokens = append(acc.rawTokens, rec.RawTokens)
	acc.filteredTokens = append(acc.filteredTokens, rec.FilteredTokens)
	acc.emittedBytes = append(acc.emittedBytes, rec.BytesEmitted)
	acc.savedPct += rec.SavingsPct
	acc.samples++
	if rec.ExitCode != 0 {
		acc.failures++
	}
	if rec.FallbackUsed {
		acc.fallbacks++
	}
}

func buildBudgetSuggestionCandidates(stats map[string]*budgetSuggestionAccumulator, minSamples int) []budgetSuggestionCandidate {
	candidates := make([]budgetSuggestionCandidate, 0, len(stats))
	for _, acc := range stats {
		if acc.samples < minSamples {
			continue
		}
		suggestion, severity, ok := buildBudgetSuggestion(acc)
		if !ok {
			continue
		}
		candidates = append(candidates, budgetSuggestionCandidate{
			suggestion: suggestion,
			severity:   severity,
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].severity == candidates[j].severity {
			if candidates[i].suggestion.Samples == candidates[j].suggestion.Samples {
				return candidates[i].suggestion.Fingerprint < candidates[j].suggestion.Fingerprint
			}
			return candidates[i].suggestion.Samples > candidates[j].suggestion.Samples
		}
		return candidates[i].severity > candidates[j].severity
	})
	return candidates
}

func limitBudgetSuggestions(candidates []budgetSuggestionCandidate, limit int) []BudgetSuggestion {
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	suggestions := make([]BudgetSuggestion, 0, len(candidates))
	for _, candidate := range candidates {
		suggestions = append(suggestions, candidate.suggestion)
	}
	return suggestions
}

func normalizeBudgetSuggestionOptions(opts BudgetSuggestionOptions) BudgetSuggestionOptions {
	if opts.Lookback <= 0 {
		opts.Lookback = 200
	}
	if opts.MinSamples <= 0 {
		opts.MinSamples = 3
	}
	return opts
}

// suggestionMaxLoosenFailureRate blocks loosen recommendations for
// fingerprints whose runs almost always exit nonzero. A fallback-heavy or
// aggressive-compression signal on such a fingerprint says "this command
// errors", not "the budget is tight", so loosening would only surface more
// error output. History records carry just the numeric ExitCode, so
// benign-by-design failures (for example grep exiting 1 on no match)
// cannot be told apart from real ones; this rate threshold is the
// pragmatic filter.
const suggestionMaxLoosenFailureRate = 80.0

func buildBudgetSuggestion(acc *budgetSuggestionAccumulator) (BudgetSuggestion, float64, bool) {
	averageSavings := acc.savedPct / float64(acc.samples)
	fallbackRate := percent(acc.fallbacks, acc.samples)
	failureRate := percent(acc.failures, acc.samples)
	medianRaw := percentileInts(acc.rawTokens, 50)
	p95Raw := percentileInts(acc.rawTokens, 95)
	medianFiltered := percentileInts(acc.filteredTokens, 50)
	p95Filtered := percentileInts(acc.filteredTokens, 95)
	medianBytes := percentileInts(acc.emittedBytes, 50)
	p95Bytes := percentileInts(acc.emittedBytes, 95)

	evidence := BudgetSuggestionEvidence{
		AverageSavingsPct:    averageSavings,
		FallbackRate:         fallbackRate,
		FailureRate:          failureRate,
		MedianRawTokens:      medianRaw,
		P95RawTokens:         p95Raw,
		MedianFilteredTokens: medianFiltered,
		P95FilteredTokens:    p95Filtered,
		MedianBytesEmitted:   medianBytes,
		P95BytesEmitted:      p95Bytes,
	}

	profile := dominantConfidence(acc.profileCounts)

	switch {
	case fallbackRate >= 50 && failureRate <= suggestionMaxLoosenFailureRate:
		scale := 1.25
		if fallbackRate >= 75 {
			scale = 1.5
		}
		targetTokens := maxInt(scaleIntCeil(p95Filtered, int(scale*100), 100), maxInt(medianFiltered+8, 24))
		targetBytes := maxInt(scaleIntCeil(p95Bytes, int(scale*100), 100), maxInt(medianBytes+32, 96))
		return finalizeBudgetSuggestion(acc, profile, evidence, BudgetSuggestionLoosen, BudgetSuggestionFallbackHeavy, scale, targetTokens, targetBytes), fallbackRate + failureRate/4, true
	case averageSavings >= 92 && medianRaw >= 80 && medianFiltered <= 20 && failureRate <= suggestionMaxLoosenFailureRate:
		scale := 1.5
		targetTokens := maxInt(scaleIntCeil(p95Filtered, 3, 2), maxInt(medianRaw/6, 24))
		targetBytes := maxInt(scaleIntCeil(p95Bytes, 3, 2), maxInt(targetTokens*4, 96))
		severity := averageSavings - 80 + float64(medianRaw)/16
		return finalizeBudgetSuggestion(acc, profile, evidence, BudgetSuggestionLoosen, BudgetSuggestionAggressiveCompression, scale, targetTokens, targetBytes), severity, true
	case averageSavings <= 35 && medianFiltered >= 48 && fallbackRate <= 25:
		scale := 0.75
		targetTokens := maxInt(scaleIntCeil(p95Filtered, 3, 4), 24)
		targetBytes := maxInt(scaleIntCeil(p95Bytes, 3, 4), 96)
		severity := 35 - averageSavings + float64(medianFiltered)/4
		return finalizeBudgetSuggestion(acc, profile, evidence, BudgetSuggestionTighten, BudgetSuggestionNoisy, scale, targetTokens, targetBytes), severity, true
	default:
		return BudgetSuggestion{}, 0, false
	}
}

func finalizeBudgetSuggestion(
	acc *budgetSuggestionAccumulator,
	profile string,
	evidence BudgetSuggestionEvidence,
	direction BudgetSuggestionDirection,
	reason BudgetSuggestionReason,
	scale float64,
	targetTokens, targetBytes int,
) BudgetSuggestion {
	target := BudgetTarget{
		MaxTokens: maxInt(targetTokens, 1),
		MaxBytes:  maxInt(targetBytes, 1),
	}
	target.MaxLines = estimateBudgetLines(target.MaxBytes, target.MaxTokens)
	return BudgetSuggestion{
		Fingerprint: acc.fingerprint,
		Command:     acc.command,
		Profile:     profile,
		Samples:     acc.samples,
		Direction:   direction,
		Reason:      reason,
		Confidence:  suggestionConfidence(acc.samples),
		Scale:       scale,
		Suggested:   target,
		Evidence:    evidence,
		FirstSeen:   acc.firstSeen,
		LastSeen:    acc.lastSeen,
	}
}
