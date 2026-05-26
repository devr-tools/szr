package workflows

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/rewrite"
)

func RunRecommend(rt Runtime, args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: recommend requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(rt.Stderr, "szr: invalid recommend limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown recommend flag %s\n", args[i])
			return 2
		}
	}

	records, err := rt.History.LoadAll()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	recommendations := BuildRecommendations(records, limit)
	if asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(recommendations)
		return 0
	}
	if len(recommendations) == 0 {
		fmt.Fprintln(rt.Stdout, "no recommendations yet")
		return 0
	}

	fmt.Fprintln(rt.Stdout, "recommendations:")
	for _, item := range recommendations {
		fmt.Fprintf(rt.Stdout, "  - [%s] %s\n", item.Kind, item.Command)
		fmt.Fprintf(rt.Stdout, "    reason: %s\n", item.Reason)
		fmt.Fprintf(rt.Stdout, "    action: %s\n", item.Action)
		if item.Profile != "" || item.Samples > 0 || item.Confidence != "" {
			fmt.Fprintf(rt.Stdout, "    profile=%s samples=%d confidence=%s\n", item.Profile, item.Samples, emptyDash(item.Confidence))
		}
		if item.Direction != "" {
			fmt.Fprintf(
				rt.Stdout,
				"    target: %s lines=%d bytes=%d tokens=%d\n",
				item.Direction,
				item.Suggested.MaxLines,
				item.Suggested.MaxBytes,
				item.Suggested.MaxTokens,
			)
		}
	}
	return 0
}

func RunHotspots(rt Runtime, args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: hotspots requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(rt.Stderr, "szr: invalid hotspots limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown hotspots flag %s\n", args[i])
			return 2
		}
	}

	records, err := rt.History.LoadAll()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	hotspots := BuildHotspots(records, limit)
	if asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(hotspots)
		return 0
	}
	if len(hotspots) == 0 {
		fmt.Fprintln(rt.Stdout, "no hotspots yet")
		return 0
	}

	fmt.Fprintln(rt.Stdout, "hotspots:")
	for _, item := range hotspots {
		fmt.Fprintf(
			rt.Stdout,
			"  - %s  profile=%s samples=%d avg=%.1f%% fallback=%.1f%% fail=%.1f%% tee=%.1f%% p50/p95=%d/%dms\n",
			item.Command,
			item.Profile,
			item.Samples,
			item.AveragePct,
			item.FallbackRate,
			item.FailureRate,
			item.TeeRate,
			item.DurationP50MS,
			item.DurationP95MS,
		)
	}
	return 0
}

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

func recommendationForBudget(suggestion history.BudgetSuggestion) Recommendation {
	return Recommendation{
		Kind:        "budget",
		Priority:    recommendationPriorityForBudget(suggestion),
		Command:     suggestion.Command,
		Profile:     suggestion.Profile,
		Samples:     suggestion.Samples,
		Confidence:  suggestion.Confidence,
		Reason:      strings.ReplaceAll(string(suggestion.Reason), "_", " "),
		Action:      fmt.Sprintf("adjust the active budget to lines=%d bytes=%d tokens=%d", suggestion.Suggested.MaxLines, suggestion.Suggested.MaxBytes, suggestion.Suggested.MaxTokens),
		Fingerprint: suggestion.Fingerprint,
		Direction:   string(suggestion.Direction),
		Suggested:   suggestion.Suggested,
	}
}

func appendHotspotRecommendations(items *[]Recommendation, seen map[string]struct{}, hotspot HotspotStat) {
	if hotspot.Samples < 2 {
		return
	}
	for _, item := range hotspotRecommendations(hotspot) {
		key := recommendationKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		*items = append(*items, item)
		seen[key] = struct{}{}
	}
}

func hotspotRecommendations(hotspot HotspotStat) []Recommendation {
	items := []Recommendation{}
	if customProfileRecommendation(hotspot).Kind != "" {
		items = append(items, customProfileRecommendation(hotspot))
	}
	if item, ok := directRoutingRecommendation(hotspot); ok {
		items = append(items, item)
	}
	if item, ok := structuredRewriteRecommendation(hotspot); ok {
		items = append(items, item)
	}
	if item, ok := wrapperGuidanceRecommendation(hotspot); ok {
		items = append(items, item)
	}
	if item, ok := teeReviewRecommendation(hotspot); ok {
		items = append(items, item)
	}
	return items
}

func customProfileRecommendation(hotspot HotspotStat) Recommendation {
	if !isGenericHotspot(hotspot) || hotspot.FallbackRate < 0 {
		return Recommendation{}
	}
	return Recommendation{
		Kind:        "custom-profile",
		Priority:    70,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      fmt.Sprintf("%s still routes through %s after %d runs", hotspot.Command, hotspot.Profile, hotspot.Samples),
		Action:      "add a project-local profile or builtin reducer so this command stops relying on the generic path",
		Fingerprint: hotspot.Fingerprint,
	}
}

func structuredRewriteRecommendation(hotspot HotspotStat) (Recommendation, bool) {
	if !isGenericHotspot(hotspot) {
		return Recommendation{}, false
	}
	hint := structuredHint(hotspot.Command)
	if hint == "" {
		return Recommendation{}, false
	}
	return Recommendation{
		Kind:        "structured-rewrite",
		Priority:    65,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      "this command family usually benefits from a deterministic machine-readable mode",
		Action:      hint,
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func directRoutingRecommendation(hotspot HotspotStat) (Recommendation, bool) {
	if !isGenericHotspot(hotspot) {
		return Recommendation{}, false
	}
	decision := rewrite.Analyze(hotspot.Command, "szr")
	if !decision.AutoRewrite || decision.Rewrite == "" || hotspot.Samples < 2 {
		return Recommendation{}, false
	}
	return Recommendation{
		Kind:        "routing-coverage",
		Priority:    68,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "high",
		Reason:      "this command family is already safe to rewrite, but history shows it still bypasses szr",
		Action:      fmt.Sprintf("route this family through szr by default; e.g. `%s`", decision.Rewrite),
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func teeReviewRecommendation(hotspot HotspotStat) (Recommendation, bool) {
	if hotspot.Failures <= 0 || hotspot.TeeRate < 50 {
		return Recommendation{}, false
	}
	return Recommendation{
		Kind:        "tee-review",
		Priority:    50,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "low",
		Reason:      "failing runs are frequently preserving full artifacts",
		Action:      fmt.Sprintf("inspect preserved failures with `szr tee find %q` before adding another reducer", firstWordOrCommand(hotspot.Command)),
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func wrapperGuidanceRecommendation(hotspot HotspotStat) (Recommendation, bool) {
	if !isGenericHotspot(hotspot) {
		return Recommendation{}, false
	}
	decision := rewrite.Analyze(hotspot.Command, "szr")
	if decision.AutoRewrite || decision.Hint == "" {
		return Recommendation{}, false
	}
	if hotspot.AveragePct > 25 && hotspot.FallbackRate == 0 && hotspot.FailureRate == 0 {
		return Recommendation{}, false
	}
	return Recommendation{
		Kind:        "wrapper-guidance",
		Priority:    60,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      "this command family is noisy, but auto-rewriting it would risk changing shell semantics",
		Action:      decision.Hint,
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func isGenericHotspot(hotspot HotspotStat) bool {
	return hotspot.Profile == "passthrough" || strings.HasPrefix(hotspot.Profile, "generic-")
}

func recommendationKey(item Recommendation) string {
	return item.Kind + ":" + item.Fingerprint
}

func routingExpansionRecommendations(hotspots []HotspotStat) []Recommendation {
	type familyAggregate struct {
		samples int
		count   int
		rep     HotspotStat
	}
	grouped := map[string]*familyAggregate{}
	for _, hotspot := range hotspots {
		if !isGenericHotspot(hotspot) {
			continue
		}
		family := rewrite.Family(hotspot.Command)
		if family == "" {
			continue
		}
		acc := grouped[family]
		if acc == nil {
			acc = &familyAggregate{rep: hotspot}
			grouped[family] = acc
		}
		acc.samples += hotspot.Samples
		acc.count++
		if hotspotSeverity(hotspot) > hotspotSeverity(acc.rep) {
			acc.rep = hotspot
		}
	}

	items := make([]Recommendation, 0, len(grouped))
	for family, acc := range grouped {
		if acc.samples < 3 || (acc.count < 2 && acc.samples < 4) {
			continue
		}
		decision := rewrite.Analyze(acc.rep.Command, "szr")
		if !decision.AutoRewrite || decision.Rewrite == "" {
			continue
		}
		items = append(items, Recommendation{
			Kind:        "routing-expansion",
			Priority:    72,
			Command:     family,
			Profile:     acc.rep.Profile,
			Samples:     acc.samples,
			Confidence:  "high",
			Reason:      "multiple history hotspots show this family repeatedly bypassing szr despite a safe rewrite path",
			Action:      fmt.Sprintf("expand default routing for this family; representative rewrite: `%s`", decision.Rewrite),
			Fingerprint: "family:" + family,
		})
	}
	return items
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

func structuredHint(command string) string {
	normalized := strings.Fields(strings.ToLower(command))
	if len(normalized) == 0 {
		return ""
	}
	if normalized[0] == "szr" && len(normalized) > 1 {
		normalized = normalized[1:]
	}
	if len(normalized) == 0 {
		return ""
	}
	switch normalized[0] {
	case "terraform", "tofu":
		return "prefer JSON-capable flows like `plan -json`, `show -json`, or structured state output via a project preference"
	case "gh":
		return "prefer `--json` with explicit fields or list/view subcommands that emit stable machine-readable output"
	case "eslint":
		return "prefer `-f json` so szr can reduce file- and rule-level diagnostics deterministically"
	case "tsc":
		return "prefer `--pretty false` and narrower file targets so output is stable and reducer-friendly"
	case "kubectl":
		return "prefer `-o json` or other structured output where the subcommand supports it"
	}
	if len(normalized) >= 2 && normalized[0] == "docker" && normalized[1] == "build" {
		return "prefer deterministic plain progress or explicit metadata flags before relying on the generic reducer"
	}
	return ""
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func firstWordOrCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command
	}
	return fields[0]
}
