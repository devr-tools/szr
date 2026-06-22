package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const (
	compressionContractMinRawTokens = 40
	compressionContractMinTokens    = 8
	compressionContractRetainedNum  = 1
	compressionContractRetainedDen  = 5
)

func enforceCompressionContract(text string, rawCombined string, budget OutputBudget, plan RecoveryPlan, passthrough bool, enabled bool) (string, RecoveryPlan, bool) {
	if passthrough || !enabled {
		return text, plan, false
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if rawTokens < compressionContractMinRawTokens {
		return text, plan, false
	}
	allowedTokens := compressionContractAllowedTokens(rawTokens, budget)
	filteredTokens := history.EstimateTokens(text)
	if allowedTokens <= 0 || filteredTokens <= allowedTokens {
		return text, plan, false
	}
	compressed := hardCapTokens(text, allowedTokens)
	if strings.TrimSpace(compressed) == "" {
		return text, plan, false
	}
	updatedPlan := plan
	updatedPlan.Kind = RecoveryKindFullOutput
	updatedPlan.RequireRawCapture = strings.TrimSpace(rawCombined) != ""
	updatedPlan.Summary = compressionRecoverySummary(plan.Summary, allowedTokens, rawTokens)
	return compressed, updatedPlan, true
}

func compressionContractAllowedTokens(rawTokens int, budget OutputBudget) int {
	if rawTokens <= 0 {
		return 0
	}
	allowed := compressionScaleIntCeil(rawTokens, compressionContractRetainedNum, compressionContractRetainedDen)
	if allowed < compressionContractMinTokens {
		allowed = compressionContractMinTokens
	}
	if budget.MaxTokens > 0 && budget.MaxTokens < allowed {
		allowed = budget.MaxTokens
	}
	if allowed < 1 {
		return 1
	}
	return allowed
}

func hardCapTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	normalized := strings.Join(fields, " ")
	if history.EstimateTokens(normalized) <= maxTokens {
		return normalized
	}
	if maxTokens == 1 {
		return "..."
	}

	selected := selectCompressionRanges(fields, maxTokens)
	if len(selected) > 0 {
		candidate := buildCompressedFromRanges(fields, selected)
		if strings.TrimSpace(candidate) != "" && history.EstimateTokens(candidate) <= maxTokens {
			return candidate
		}
	}
	for keep := len(fields); keep > 0; keep-- {
		candidate := buildCompressedFromRanges(fields, []compressionRange{{start: 0, end: keep}})
		if history.EstimateTokens(candidate) <= maxTokens {
			return candidate
		}
	}
	return "..."
}

type compressionRange struct {
	start int
	end   int
	score int
}

func selectCompressionRanges(fields []string, maxTokens int) []compressionRange {
	candidates := compressionCandidateRanges(fields)
	selected := make([]compressionRange, 0, 4)
	for _, candidate := range candidates {
		if compressionRangeCovered(selected, candidate) {
			continue
		}
		trial := append(append([]compressionRange{}, selected...), candidate)
		trial = mergeCompressionRanges(trial)
		if history.EstimateTokens(buildCompressedFromRanges(fields, trial)) <= maxTokens {
			selected = trial
		}
	}
	return mergeCompressionRanges(selected)
}

//nolint:maintidx // This range builder is intentionally centralised so token-aware compression decisions stay coherent.
func compressionCandidateRanges(fields []string) []compressionRange {
	if len(fields) == 0 {
		return nil
	}

	builder := newCompressionRangeBuilder(fields)
	builder.addEdgeRanges()
	builder.addAnchorRanges()
	builder.addClusterRanges()
	candidates := builder.candidates()
	sort.Slice(candidates, func(i, j int) bool {
		leftWidth := candidates[i].end - candidates[i].start
		rightWidth := candidates[j].end - candidates[j].start
		leftDensity := candidates[i].score * 100 / compressionMaxInt(leftWidth, 1)
		rightDensity := candidates[j].score * 100 / compressionMaxInt(rightWidth, 1)
		if leftDensity != rightDensity {
			return leftDensity > rightDensity
		}
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].start != candidates[j].start {
			return candidates[i].start < candidates[j].start
		}
		return candidates[i].end > candidates[j].end
	})
	return candidates
}

const (
	compressionContextBefore = 2
	compressionContextAfter  = 3
	compressionTightBefore   = 1
	compressionTightAfter    = 2
	compressionEdgeWindow    = 5
)

type compressionRangeBuilder struct {
	fields        []string
	ranges        map[[2]int]compressionRange
	anchorIndexes []int
}

func newCompressionRangeBuilder(fields []string) *compressionRangeBuilder {
	return &compressionRangeBuilder{
		fields:        fields,
		ranges:        map[[2]int]compressionRange{},
		anchorIndexes: make([]int, 0, len(fields)),
	}
}

func (b *compressionRangeBuilder) addEdgeRanges() {
	b.addRange(0, minInt(len(b.fields), compressionEdgeWindow), 8)
	if len(b.fields) > compressionEdgeWindow {
		b.addRange(len(b.fields)-compressionEdgeWindow, len(b.fields), 6)
	}
}

func (b *compressionRangeBuilder) addAnchorRanges() {
	for i, field := range b.fields {
		score := compressionAnchorScore(field)
		if score == 0 {
			continue
		}
		b.anchorIndexes = append(b.anchorIndexes, i)
		b.addRange(i-compressionTightBefore, i+compressionTightAfter, score+8)
		b.addRange(i-compressionContextBefore, i+compressionContextAfter, score)
	}
}

func (b *compressionRangeBuilder) addClusterRanges() {
	for clusterStart := 0; clusterStart < len(b.anchorIndexes); {
		clusterEnd := b.clusterEnd(clusterStart)
		start, end := b.clusterBounds(clusterStart, clusterEnd)
		b.addRange(start, end, b.clusterScore(clusterStart, clusterEnd))
		clusterStart = clusterEnd
	}
}

func (b *compressionRangeBuilder) clusterEnd(start int) int {
	end := start + 1
	for end < len(b.anchorIndexes) && b.anchorIndexes[end]-b.anchorIndexes[end-1] <= 3 {
		end++
	}
	return end
}

func (b *compressionRangeBuilder) clusterBounds(start, end int) (int, int) {
	return b.anchorIndexes[start] - compressionTightBefore, b.anchorIndexes[end-1] + compressionTightAfter
}

func (b *compressionRangeBuilder) clusterScore(start, end int) int {
	score := 12
	for i := start; i < end; i++ {
		score += compressionAnchorScore(b.fields[b.anchorIndexes[i]])
	}
	return score
}

func (b *compressionRangeBuilder) addRange(start int, end int, score int) {
	start = compressionClamp(start, 0, len(b.fields))
	end = compressionClamp(end, 0, len(b.fields))
	if start >= end {
		return
	}
	key := [2]int{start, end}
	candidate := compressionRange{
		start: start,
		end:   end,
		score: score + compressionRangeScore(b.fields[start:end]),
	}
	if existing, ok := b.ranges[key]; !ok || candidate.score > existing.score {
		b.ranges[key] = candidate
	}
}

func (b *compressionRangeBuilder) candidates() []compressionRange {
	candidates := make([]compressionRange, 0, len(b.ranges))
	for _, candidate := range b.ranges {
		candidates = append(candidates, candidate)
	}
	return candidates
}

func compressionClamp(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func compressionRangeScore(fields []string) int {
	score := 0
	for _, field := range fields {
		anchor := compressionAnchorScore(field)
		if anchor > 0 {
			score += anchor
			continue
		}
		score += compressionContextScore(field)
	}
	return score
}

func compressionAnchorScore(field string) int {
	token := strings.TrimSpace(field)
	if token == "" {
		return 0
	}

	trimmed := strings.Trim(token, "[](){}<>,;:!?'\"")
	lower := strings.ToLower(trimmed)
	score := compressionKeywordScore(lower)
	score += compressionFlagScore(lower)
	score += compressionShapeScore(token, trimmed)
	return score
}

func compressionKeywordScore(lower string) int {
	switch lower {
	case "error", "errors", "failed", "failure", "panic", "fatal", "undefined", "exception", "traceback":
		return 40
	case "retry", "rerun", "re-run", "hint", "inspect", "debug", "fix":
		return 14
	default:
		return 0
	}
}

func compressionFlagScore(lower string) int {
	if strings.HasPrefix(lower, "--") || strings.HasPrefix(lower, "-") {
		return 24
	}
	return 0
}

func compressionShapeScore(token string, trimmed string) int {
	score := 0
	if looksLikePathToken(trimmed) {
		score += 28
	}
	if looksLikeIdentifierToken(trimmed) {
		score += 18
	}
	if strings.ContainsAny(token, "=:#[]{}()") || strings.Contains(token, "->") || strings.Contains(token, "::") {
		score += 10
	}
	return score
}

func compressionContextScore(field string) int {
	token := strings.Trim(field, "[](){}<>,;:!?'\"")
	if token == "" {
		return 0
	}
	if strings.Contains(strings.ToLower(token), "warning") {
		return 4
	}
	if len(token) >= 8 {
		return 2
	}
	return 1
}

func looksLikePathToken(token string) bool {
	if token == "" {
		return false
	}
	lower := strings.ToLower(token)
	return looksLikePathSeparatorToken(token) || looksLikeLineAddressToken(token) || hasPathSuffix(lower)
}

func looksLikePathSeparatorToken(token string) bool {
	return strings.Contains(token, "/") || strings.Contains(token, "\\")
}

func looksLikeLineAddressToken(token string) bool {
	for i := 0; i < len(token); i++ {
		if token[i] != ':' {
			continue
		}
		if i+1 < len(token) && token[i+1] >= '0' && token[i+1] <= '9' {
			return true
		}
	}
	return false
}

func hasPathSuffix(lower string) bool {
	for _, suffix := range []string{".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".yaml", ".yml", ".md", ".txt", ".log", ".sh"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func looksLikeIdentifierToken(token string) bool {
	if token == "" {
		return false
	}
	shape := identifierTokenShape(token)
	hasAlpha := shape.hasLower || shape.hasUpper
	if shape.hasConnector && hasAlpha {
		return true
	}
	return shape.hasLower && shape.hasUpper || shape.hasDigit && hasAlpha
}

type identifierShape struct {
	hasLower     bool
	hasUpper     bool
	hasDigit     bool
	hasConnector bool
}

func identifierTokenShape(token string) identifierShape {
	shape := identifierShape{}
	for i := 0; i < len(token); i++ {
		switch ch := token[i]; {
		case ch >= 'a' && ch <= 'z':
			shape.hasLower = true
		case ch >= 'A' && ch <= 'Z':
			shape.hasUpper = true
		case ch >= '0' && ch <= '9':
			shape.hasDigit = true
		case ch == '_' || ch == '.' || ch == '-':
			shape.hasConnector = true
		}
	}
	return shape
}

func compressionRangeCovered(existing []compressionRange, candidate compressionRange) bool {
	for _, item := range existing {
		if item.start <= candidate.start && item.end >= candidate.end {
			return true
		}
	}
	return false
}

func mergeCompressionRanges(ranges []compressionRange) []compressionRange {
	if len(ranges) == 0 {
		return nil
	}
	sorted := sortedCompressionRanges(ranges)
	return mergedSortedCompressionRanges(sorted)
}

func sortedCompressionRanges(ranges []compressionRange) []compressionRange {
	sorted := append([]compressionRange{}, ranges...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].start != sorted[j].start {
			return sorted[i].start < sorted[j].start
		}
		return sorted[i].end < sorted[j].end
	})
	return sorted
}

func mergedSortedCompressionRanges(sorted []compressionRange) []compressionRange {
	merged := []compressionRange{sorted[0]}
	for _, current := range sorted[1:] {
		if mergeIntoLastCompressionRange(&merged[len(merged)-1], current) {
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

func mergeIntoLastCompressionRange(last *compressionRange, current compressionRange) bool {
	if current.start > last.end {
		return false
	}
	if current.end > last.end {
		last.end = current.end
	}
	if current.score > last.score {
		last.score = current.score
	}
	return true
}

func buildCompressedFromRanges(fields []string, ranges []compressionRange) string {
	if len(fields) == 0 || len(ranges) == 0 {
		return ""
	}
	merged := mergeCompressionRanges(ranges)
	parts := make([]string, 0, len(fields))
	if merged[0].start > 0 {
		parts = append(parts, "...")
	}
	for i, current := range merged {
		if i > 0 && merged[i-1].end < current.start {
			parts = append(parts, "...")
		}
		parts = append(parts, fields[current.start:current.end]...)
	}
	if merged[len(merged)-1].end < len(fields) {
		parts = append(parts, "...")
	}
	return strings.Join(parts, " ")
}

func compressionScaleIntCeil(value int, num int, den int) int {
	if value <= 0 || num <= 0 || den <= 0 {
		return value
	}
	return (value*num + den - 1) / den
}

func compressionMaxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func compressionRecoverySummary(existing string, allowedTokens int, rawTokens int) string {
	summary := fmt.Sprintf("compressed to %d tokens from %d", allowedTokens, rawTokens)
	if strings.TrimSpace(existing) == "" {
		return summary
	}
	return existing + "; " + summary
}
