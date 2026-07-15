package history

import "sort"

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
		profile.stat.EmptyResultRate = percent(profile.stat.EmptyResults, profile.stat.Commands)
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
