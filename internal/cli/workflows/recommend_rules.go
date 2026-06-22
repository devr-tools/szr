package workflows

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/rewrite"
)

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
		Priority:    70 + hotspotPriorityBoost(hotspot, 12),
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      fmt.Sprintf("%s still relies on %s with %s", hotspot.Command, hotspot.Profile, hotspotSignalsSummary(hotspot)),
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
		Priority:    65 + hotspotPriorityBoost(hotspot, 8),
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      fmt.Sprintf("history shows %s and this command family usually benefits from a deterministic machine-readable mode", hotspotSignalsSummary(hotspot)),
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
		Priority:    68 + hotspotPriorityBoost(hotspot, 10),
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "high",
		Reason:      fmt.Sprintf("this command family is already safe to rewrite, but history still shows %s", hotspotSignalsSummary(hotspot)),
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
		Priority:    50 + hotspotPriorityBoost(hotspot, 6),
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "low",
		Reason:      fmt.Sprintf("history shows %s, so preserved artifacts are masking reducer gaps", hotspotSignalsSummary(hotspot)),
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
		Priority:    60 + hotspotPriorityBoost(hotspot, 8),
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      fmt.Sprintf("history shows %s, but auto-rewriting this command would risk changing shell semantics", hotspotSignalsSummary(hotspot)),
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
			Priority:    72 + hotspotPriorityBoost(acc.rep, 10),
			Command:     family,
			Profile:     acc.rep.Profile,
			Samples:     acc.samples,
			Confidence:  "high",
			Reason:      fmt.Sprintf("multiple history hotspots show %s for this family despite a safe rewrite path", hotspotSignalsSummary(acc.rep)),
			Action:      fmt.Sprintf("expand default routing for this family; representative rewrite: `%s`", decision.Rewrite),
			Fingerprint: "family:" + family,
		})
	}
	return items
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
