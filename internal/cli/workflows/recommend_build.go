package workflows

import (
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

func BuildRecommendations(records []history.Record, limit int) []Recommendation {
	if len(records) == 0 {
		return nil
	}
	hotspots := BuildHotspots(records, limit*2)
	suggestions := history.SuggestBudgets(records, history.BudgetSuggestionOptions{Limit: limit * 2})
	items := make([]Recommendation, 0, len(suggestions)+len(hotspots))
	seen := map[string]struct{}{}

	for _, suggestion := range suggestions {
		item := recommendationForBudget(suggestion)
		items = append(items, item)
		seen[recommendationKey(item)] = struct{}{}
	}

	for _, hotspot := range hotspots {
		appendHotspotRecommendations(&items, seen, hotspot)
	}
	for _, item := range routingExpansionRecommendations(hotspots) {
		key := recommendationKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		items = append(items, item)
		seen[key] = struct{}{}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			if items[i].Samples == items[j].Samples {
				return items[i].Command < items[j].Command
			}
			return items[i].Samples > items[j].Samples
		}
		return items[i].Priority > items[j].Priority
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func BuildHotspots(records []history.Record, limit int) []HotspotStat {
	if len(records) == 0 {
		return nil
	}
	type aggregate struct {
		stat      HotspotStat
		durations []int64
	}
	grouped := map[string]*aggregate{}
	for _, raw := range records {
		rec := raw
		if strings.TrimSpace(rec.CommandFingerprint) == "" {
			rec.CommandFingerprint = history.Fingerprint(rec.Command)
		}
		if rec.CommandFingerprint == "" {
			continue
		}
		item := grouped[rec.CommandFingerprint]
		if item == nil {
			item = &aggregate{stat: HotspotStat{
				Fingerprint: rec.CommandFingerprint,
				Command:     rec.Command,
				Profile:     rec.Profile,
			}}
			grouped[rec.CommandFingerprint] = item
		}
		item.stat.Samples++
		item.stat.AveragePct += rec.SavingsPct
		if rec.ExitCode != 0 {
			item.stat.Failures++
		}
		if rec.FallbackUsed {
			item.stat.Fallbacks++
		}
		if rec.TeePath != "" {
			item.stat.TeeCount++
		}
		item.durations = append(item.durations, rec.DurationMS)
	}

	list := make([]HotspotStat, 0, len(grouped))
	for _, item := range grouped {
		item.stat.AveragePct /= float64(item.stat.Samples)
		item.stat.FailureRate = percent(item.stat.Failures, item.stat.Samples)
		item.stat.FallbackRate = percent(item.stat.Fallbacks, item.stat.Samples)
		item.stat.TeeRate = percent(item.stat.TeeCount, item.stat.Samples)
		item.stat.DurationP50MS = percentileInt64(item.durations, 50)
		item.stat.DurationP95MS = percentileInt64(item.durations, 95)
		list = append(list, item.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		leftScore := hotspotSeverity(list[i])
		rightScore := hotspotSeverity(list[j])
		if leftScore == rightScore {
			if list[i].Samples == list[j].Samples {
				return list[i].Command < list[j].Command
			}
			return list[i].Samples > list[j].Samples
		}
		return leftScore > rightScore
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

func hotspotSeverity(item HotspotStat) float64 {
	return (100 - item.AveragePct) + item.FallbackRate + item.FailureRate/2 + item.TeeRate/4 + float64(item.DurationP95MS)/25
}

func recommendationPriorityForBudget(item history.BudgetSuggestion) int {
	switch item.Reason {
	case history.BudgetSuggestionFallbackHeavy:
		return 90
	case history.BudgetSuggestionAggressiveCompression:
		return 80
	default:
		return 75
	}
}
