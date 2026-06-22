package workflows

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/rewrite"
)

type DiscoverSummary struct {
	Records             int `json:"records"`
	RecommendationCount int `json:"recommendation_count"`
	HotspotCount        int `json:"hotspot_count"`
}

type DiscoverOpportunity struct {
	Recommendation
	Signals       []string `json:"signals,omitempty"`
	CoverageScore int      `json:"coverage_score,omitempty"`
	AveragePct    float64  `json:"average_pct,omitempty"`
	FallbackRate  float64  `json:"fallback_rate,omitempty"`
	FailureRate   float64  `json:"failure_rate,omitempty"`
	TeeRate       float64  `json:"tee_rate,omitempty"`
	DurationP95MS int64    `json:"duration_p95_ms,omitempty"`
}

type DiscoverReport struct {
	Summary       DiscoverSummary       `json:"summary"`
	Opportunities []DiscoverOpportunity `json:"opportunities"`
}

func RunDiscover(rt Runtime, args []string) int {
	asJSON, limit, code := parseDiscoverArgs(rt, args)
	if code != 0 {
		return code
	}
	records, err := rt.History.LoadAll()
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	report := BuildDiscover(records, limit)
	if asJSON {
		writeDiscoverJSON(rt, report)
		return 0
	}
	return writeDiscoverText(rt, report)
}

func BuildDiscover(records []history.Record, limit int) DiscoverReport {
	report := DiscoverReport{Summary: DiscoverSummary{Records: len(records)}}
	if len(records) == 0 {
		return report
	}

	hotspots := BuildHotspots(records, limit*2)
	recommendations := BuildRecommendations(records, limit*2)
	report.Summary.HotspotCount = len(hotspots)
	report.Summary.RecommendationCount = len(recommendations)
	index := buildDiscoverHotspotIndex(hotspots)
	report.Opportunities = limitDiscoverOpportunities(buildDiscoverOpportunities(recommendations, index), limit)
	return report
}

type discoverHotspotIndex struct {
	byFingerprint map[string]HotspotStat
	byFamily      map[string]HotspotStat
}

func parseDiscoverArgs(rt Runtime, args []string) (bool, int, int) {
	asJSON := false
	limit := 5
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			value, next, ok := parseDiscoverLimit(rt, args, i)
			if !ok {
				return false, 0, 2
			}
			limit = value
			i = next
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown discover flag %s\n", args[i])
			return false, 0, 2
		}
	}
	return asJSON, limit, 0
}

func parseDiscoverLimit(rt Runtime, args []string, index int) (int, int, bool) {
	if index+1 >= len(args) {
		fmt.Fprintln(rt.Stderr, "szr: discover requires a value after --limit")
		return 0, index, false
	}
	value, err := strconv.Atoi(args[index+1])
	if err != nil || value <= 0 {
		fmt.Fprintf(rt.Stderr, "szr: invalid discover limit %q\n", args[index+1])
		return 0, index, false
	}
	return value, index + 1, true
}

func writeDiscoverJSON(rt Runtime, report DiscoverReport) {
	enc := json.NewEncoder(rt.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

func writeDiscoverText(rt Runtime, report DiscoverReport) int {
	if len(report.Opportunities) == 0 {
		fmt.Fprintln(rt.Stdout, "no discovery opportunities yet")
		return 0
	}
	fmt.Fprintf(rt.Stdout, "discover: top %d of %d opportunities across %d hotspots\n", len(report.Opportunities), report.Summary.RecommendationCount, report.Summary.HotspotCount)
	for _, item := range report.Opportunities {
		writeDiscoverOpportunity(rt, item)
	}
	return 0
}

func writeDiscoverOpportunity(rt Runtime, item DiscoverOpportunity) {
	fmt.Fprintf(rt.Stdout, "  - [%s] %s\n", item.Kind, item.Command)
	fmt.Fprintf(rt.Stdout, "    do: %s\n", item.Action)
	fmt.Fprintf(rt.Stdout, "    why: %s\n", item.Reason)
	fmt.Fprintf(rt.Stdout, "    %s\n", discoverOpportunityMetadata(item))
	if len(item.Signals) > 0 {
		fmt.Fprintf(rt.Stdout, "    signals=%s avg=%.1f%% fallback=%.1f%% fail=%.1f%% tee=%.1f%%\n", hotspotSignalList(HotspotStat{Signals: item.Signals}), item.AveragePct, item.FallbackRate, item.FailureRate, item.TeeRate)
	}
}

func discoverOpportunityMetadata(item DiscoverOpportunity) string {
	meta := fmt.Sprintf("profile=%s samples=%d confidence=%s", emptyDash(item.Profile), item.Samples, emptyDash(item.Confidence))
	if item.CoverageScore > 0 {
		meta += fmt.Sprintf(" score=%d", item.CoverageScore)
	}
	if item.DurationP95MS > 0 {
		meta += fmt.Sprintf(" p95=%dms", item.DurationP95MS)
	}
	return meta
}

func buildDiscoverHotspotIndex(hotspots []HotspotStat) discoverHotspotIndex {
	index := discoverHotspotIndex{
		byFingerprint: make(map[string]HotspotStat, len(hotspots)),
		byFamily:      make(map[string]HotspotStat, len(hotspots)),
	}
	for _, hotspot := range hotspots {
		index.byFingerprint[hotspot.Fingerprint] = hotspot
		updateDiscoverFamilyHotspot(index.byFamily, hotspot)
	}
	return index
}

func updateDiscoverFamilyHotspot(byFamily map[string]HotspotStat, hotspot HotspotStat) {
	family := rewrite.Family(hotspot.Command)
	if family == "" {
		return
	}
	current, ok := byFamily[family]
	if !ok || hotspotSeverity(hotspot) > hotspotSeverity(current) {
		byFamily[family] = hotspot
	}
}

func buildDiscoverOpportunities(recommendations []Recommendation, index discoverHotspotIndex) []DiscoverOpportunity {
	opportunities := make([]DiscoverOpportunity, 0, len(recommendations))
	for _, item := range recommendations {
		opportunity := DiscoverOpportunity{Recommendation: item}
		if hotspot, ok := discoverHotspotForRecommendation(item, index); ok {
			applyDiscoverHotspot(&opportunity, hotspot)
		}
		opportunities = append(opportunities, opportunity)
	}
	return opportunities
}

func discoverHotspotForRecommendation(item Recommendation, index discoverHotspotIndex) (HotspotStat, bool) {
	if hotspot, ok := index.byFingerprint[item.Fingerprint]; ok {
		return hotspot, true
	}
	if item.Kind == "routing-expansion" {
		hotspot, ok := index.byFamily[item.Command]
		return hotspot, ok
	}
	return HotspotStat{}, false
}

func limitDiscoverOpportunities(opportunities []DiscoverOpportunity, limit int) []DiscoverOpportunity {
	sort.Slice(opportunities, func(i, j int) bool {
		if opportunities[i].Priority == opportunities[j].Priority {
			if opportunities[i].CoverageScore == opportunities[j].CoverageScore {
				return opportunities[i].Samples > opportunities[j].Samples
			}
			return opportunities[i].CoverageScore > opportunities[j].CoverageScore
		}
		return opportunities[i].Priority > opportunities[j].Priority
	})
	if limit > 0 && len(opportunities) > limit {
		return opportunities[:limit]
	}
	return opportunities
}

func applyDiscoverHotspot(item *DiscoverOpportunity, hotspot HotspotStat) {
	item.Signals = append([]string(nil), hotspot.Signals...)
	item.CoverageScore = hotspot.CoverageScore
	item.AveragePct = hotspot.AveragePct
	item.FallbackRate = hotspot.FallbackRate
	item.FailureRate = hotspot.FailureRate
	item.TeeRate = hotspot.TeeRate
	item.DurationP95MS = hotspot.DurationP95MS
}
