package history

import (
	"sort"
	"time"
)

type BudgetSuggestionOptions struct {
	Limit      int `json:"limit,omitempty"`
	Lookback   int `json:"lookback,omitempty"`
	MinSamples int `json:"min_samples,omitempty"`
}

type BudgetSuggestionDirection string

const (
	BudgetSuggestionTighten BudgetSuggestionDirection = "tighten"
	BudgetSuggestionLoosen  BudgetSuggestionDirection = "loosen"
)

type BudgetSuggestionReason string

const (
	BudgetSuggestionNoisy                 BudgetSuggestionReason = "noisy"
	BudgetSuggestionAggressiveCompression BudgetSuggestionReason = "aggressive_compression"
	BudgetSuggestionFallbackHeavy         BudgetSuggestionReason = "fallback_heavy"
)

type BudgetTarget struct {
	MaxLines  int `json:"max_lines"`
	MaxBytes  int `json:"max_bytes"`
	MaxTokens int `json:"max_tokens"`
}

type BudgetSuggestionEvidence struct {
	AverageSavingsPct    float64 `json:"average_savings_pct"`
	FallbackRate         float64 `json:"fallback_rate"`
	FailureRate          float64 `json:"failure_rate"`
	MedianRawTokens      int     `json:"median_raw_tokens"`
	P95RawTokens         int     `json:"p95_raw_tokens"`
	MedianFilteredTokens int     `json:"median_filtered_tokens"`
	P95FilteredTokens    int     `json:"p95_filtered_tokens"`
	MedianBytesEmitted   int     `json:"median_bytes_emitted"`
	P95BytesEmitted      int     `json:"p95_bytes_emitted"`
}

type BudgetSuggestion struct {
	Fingerprint string                    `json:"fingerprint"`
	Command     string                    `json:"command"`
	Profile     string                    `json:"profile"`
	Samples     int                       `json:"samples"`
	Direction   BudgetSuggestionDirection `json:"direction"`
	Reason      BudgetSuggestionReason    `json:"reason"`
	Confidence  string                    `json:"confidence"`
	Scale       float64                   `json:"scale"`
	Suggested   BudgetTarget              `json:"suggested"`
	Evidence    BudgetSuggestionEvidence  `json:"evidence"`
	FirstSeen   time.Time                 `json:"first_seen"`
	LastSeen    time.Time                 `json:"last_seen"`
}

type budgetSuggestionAccumulator struct {
	fingerprint    string
	command        string
	lastSeen       time.Time
	firstSeen      time.Time
	profileCounts  map[string]int
	rawTokens      []int
	filteredTokens []int
	emittedBytes   []int
	savedPct       float64
	samples        int
	failures       int
	fallbacks      int
}

type budgetSuggestionCandidate struct {
	suggestion BudgetSuggestion
	severity   float64
}

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
		if rec.CommandFingerprint == "" {
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

func buildBudgetSuggestionCandidates(
	stats map[string]*budgetSuggestionAccumulator,
	minSamples int,
) []budgetSuggestionCandidate {
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
	case fallbackRate >= 50:
		scale := 1.25
		if fallbackRate >= 75 {
			scale = 1.5
		}
		targetTokens := maxInt(scaleIntCeil(p95Filtered, int(scale*100), 100), maxInt(medianFiltered+8, 24))
		targetBytes := maxInt(scaleIntCeil(p95Bytes, int(scale*100), 100), maxInt(medianBytes+32, 96))
		return finalizeBudgetSuggestion(acc, profile, evidence, BudgetSuggestionLoosen, BudgetSuggestionFallbackHeavy, scale, targetTokens, targetBytes), fallbackRate + failureRate/4, true
	case averageSavings >= 92 && medianRaw >= 80 && medianFiltered <= 20:
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

func percentileInts(values []int, target int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	if target <= 0 {
		return sorted[0]
	}
	if target >= 100 {
		return sorted[len(sorted)-1]
	}
	index := (len(sorted)*target + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}

func estimateBudgetLines(maxBytes, maxTokens int) int {
	linesByBytes := scaleIntCeil(maxBytes, 1, 48)
	linesByTokens := scaleIntCeil(maxTokens, 1, 12)
	lines := maxInt(linesByBytes, linesByTokens)
	if lines < 3 {
		lines = 3
	}
	if lines > 40 {
		lines = 40
	}
	return lines
}

func suggestionConfidence(samples int) string {
	switch {
	case samples >= 6:
		return "high"
	case samples >= 4:
		return "medium"
	default:
		return "low"
	}
}

func scaleIntCeil(value, num, den int) int {
	if value <= 0 || num <= 0 || den <= 0 {
		return 0
	}
	return (value*num + den - 1) / den
}

func maxInt(values ...int) int {
	best := 0
	for i, value := range values {
		if i == 0 || value > best {
			best = value
		}
	}
	return best
}
