package engine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

// Fidelity floor for the compression contract. The contract exists to keep
// LARGE outputs at <=1/5 of their raw token cost; on small outputs the
// absolute savings are trivial while the fidelity cost is total (a 61-token
// lint failure crushed to 13 tokens of ellipses is zero signal). The raw
// threshold therefore only arms the contract for genuinely big outputs, and
// the retained-token floor guarantees the contract never crushes a render
// below a usable diagnostic size.
const (
	compressionContractMinRawTokens = 200
	compressionContractMinTokens    = 48
	compressionContractRetainedNum  = 1
	compressionContractRetainedDen  = 5
)

func enforceCompressionContract(text string, rawCombined string, rawTokens int, budget OutputBudget, plan RecoveryPlan, passthrough bool, enabled bool) (string, RecoveryPlan, bool) {
	if passthrough || !enabled {
		return text, plan, false
	}
	rawTokens = trueRawTokenCount(rawTokens, rawCombined)
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
	return compressed, compressionContractPlan(plan, rawCombined, allowedTokens, rawTokens), true
}

func compressionContractPlan(plan RecoveryPlan, rawCombined string, allowedTokens int, rawTokens int) RecoveryPlan {
	plan.Kind = RecoveryKindFullOutput
	plan.RequireRawCapture = strings.TrimSpace(rawCombined) != ""
	plan.Summary = compressionRecoverySummary(plan.Summary, allowedTokens, rawTokens)
	return plan
}

// trueRawTokenCount returns the raw token count the compression contract
// should budget against. Streaming runs may capture only a short preview of
// the raw output while the streamed token counter saw the full stream, so
// prefer the provided count and only fall back to estimating the captured
// text when it is the larger signal (the batch path, where rawCombined is the
// complete output).
func trueRawTokenCount(provided int, rawCombined string) int {
	if estimated := history.EstimateTokens(rawCombined); estimated > provided {
		return estimated
	}
	return provided
}

func compressionContractAllowedTokens(rawTokens int, budget OutputBudget) int {
	if rawTokens <= 0 {
		return 0
	}
	allowed := compressionScaleIntCeil(rawTokens, compressionContractRetainedNum, compressionContractRetainedDen)
	if budget.MaxTokens > 0 && budget.MaxTokens < allowed {
		allowed = budget.MaxTokens
	}
	// The usable floor is applied after the budget cap on purpose: a profile
	// budget must not be able to push the contract below the fidelity floor.
	if allowed < compressionContractMinTokens {
		allowed = compressionContractMinTokens
	}
	return allowed
}

// hardCapTokens compresses text to roughly maxTokens while preserving the
// most diagnostic content. It operates line-first: whole lines are scored
// with the shared anchor/keyword machinery and the highest-value lines are
// kept verbatim, because a render must never be less informative than the
// tokens it spends — shredding lines into word salad destroys diagnostics.
// Word-range compression is only a fallback for a single overlong line. The
// result is never content-free: when the cap would leave only ellipsis
// markers, the single highest-value line is kept even if it exceeds the cap.
func hardCapTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	lines := normalizedCompressionLines(text)
	if len(lines) == 0 {
		return ""
	}
	if candidate := strings.Join(lines, "\n"); history.EstimateTokens(candidate) <= maxTokens {
		return candidate
	}
	if kept := selectWholeLinesWithinCap(lines, maxTokens); kept != "" {
		return kept
	}
	return capSingleLine(bestCompressionLine(lines), maxTokens)
}

// normalizedCompressionLines splits text into non-blank lines with collapsed
// intra-line whitespace, preserving line boundaries (unlike the old
// whole-text strings.Fields flattening, which erased the structure the line
// scorer needs).
func normalizedCompressionLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	return lines
}

// selectWholeLinesWithinCap keeps the highest-scoring whole lines that fit
// within maxTokens, in original order, with "..." markers for elided runs.
// The first content line is always tried first: by convention across szr
// profiles it is the render's headline summary, and a capped render that
// loses its headline is strictly less informative than one that loses a
// detail line. Returns "" when not even one whole line fits.
func selectWholeLinesWithinCap(lines []string, maxTokens int) string {
	kept := make([]bool, len(lines))
	best := ""
	for _, idx := range compressionSelectionOrder(lines, maxTokens) {
		kept[idx] = true
		candidate := buildCompressedFromKeptLines(lines, kept)
		if history.EstimateTokens(candidate) <= maxTokens {
			best = candidate
			continue
		}
		kept[idx] = false
	}
	return best
}

// compressionSelectionOrder returns the whole-line candidate order with the
// headline (first content line) forced to the front so it is granted budget
// before any detail line can crowd it out. Pre-existing ellipsis markers are
// never headlines.
func compressionSelectionOrder(lines []string, maxTokens int) []int {
	order := compressionLineOrder(lines, maxTokens)
	if len(lines) == 0 || lines[0] == "..." {
		return order
	}
	reordered := make([]int, 0, len(order)+1)
	reordered = append(reordered, 0)
	for _, idx := range order {
		if idx != 0 {
			reordered = append(reordered, idx)
		}
	}
	return reordered
}

// compressionLineOrder returns line indexes ordered most-valuable-first,
// pre-trimmed to the top maxTokens candidates (each kept line costs at least
// one token, so lower-ranked lines can never be selected). Pre-existing
// ellipsis marker lines are never candidates: elision markers are re-derived
// from the gaps, so keeping stale ones would double them up.
func compressionLineOrder(lines []string, maxTokens int) []int {
	order := compressionCandidateLineIndexes(lines)
	sortCompressionLineOrder(order, lines)
	if len(order) > maxTokens {
		order = order[:maxTokens]
	}
	return order
}

func compressionCandidateLineIndexes(lines []string) []int {
	order := make([]int, 0, len(lines))
	for i, line := range lines {
		if line != "..." {
			order = append(order, i)
		}
	}
	if len(order) == 0 {
		order = append(order, 0)
	}
	return order
}

func sortCompressionLineOrder(order []int, lines []string) {
	scores := make([]int, len(lines))
	densities := make([]int, len(lines))
	for i, line := range lines {
		scores[i] = compressionLineScore(lines, i, line)
		densities[i] = scores[i] * 100 / compressionMaxInt(history.EstimateTokens(line), 1)
	}
	sort.Slice(order, func(a, b int) bool {
		left, right := order[a], order[b]
		if densities[left] != densities[right] {
			return densities[left] > densities[right]
		}
		if scores[left] != scores[right] {
			return scores[left] > scores[right]
		}
		return left < right
	})
}

func compressionLineScore(lines []string, index int, line string) int {
	score := compressionRangeScore(strings.Fields(line))
	// Mirror the edge bonuses of the word-range builder: the first and last
	// lines carry framing (headers, exit summaries) worth keeping.
	if index == 0 {
		score += 8
	}
	if index == len(lines)-1 && len(lines) > 1 {
		score += 6
	}
	return score
}

func bestCompressionLine(lines []string) string {
	order := compressionLineOrder(lines, 1)
	return lines[order[0]]
}

func buildCompressedFromKeptLines(lines []string, kept []bool) string {
	parts := make([]string, 0, len(lines))
	pendingGap := false
	for i, line := range lines {
		if !kept[i] {
			pendingGap = true
			continue
		}
		if pendingGap {
			parts = append(parts, "...")
			pendingGap = false
		}
		parts = append(parts, line)
	}
	if pendingGap {
		parts = append(parts, "...")
	}
	return strings.Join(parts, "\n")
}

// capSingleLine applies word-range compression inside one overlong line.
// The final fallback deliberately returns real content past the cap rather
// than a bare ellipsis: an over-budget line still informs, a marker never
// does.
func capSingleLine(line string, maxTokens int) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	if candidate := capSingleLineByRanges(fields, maxTokens); candidate != "" {
		return candidate
	}
	if candidate := capSingleLineByPrefix(fields, maxTokens); candidate != "" {
		return candidate
	}
	if history.EstimateTokens(line) <= 2*maxTokens {
		return line
	}
	return clipRunes(line, maxTokens*4) + " ..."
}

func capSingleLineByRanges(fields []string, maxTokens int) string {
	if maxTokens <= 1 {
		return ""
	}
	selected := selectCompressionRanges(fields, maxTokens)
	if len(selected) == 0 {
		return ""
	}
	candidate := buildCompressedFromRanges(fields, selected)
	if !isContentFreeRender(candidate) && history.EstimateTokens(candidate) <= maxTokens {
		return candidate
	}
	return ""
}

func capSingleLineByPrefix(fields []string, maxTokens int) string {
	if maxTokens <= 1 {
		return ""
	}
	for keep := len(fields); keep > 0; keep-- {
		candidate := buildCompressedFromRanges(fields, []compressionRange{{start: 0, end: keep}})
		if !isContentFreeRender(candidate) && history.EstimateTokens(candidate) <= maxTokens {
			return candidate
		}
	}
	return ""
}

func clipRunes(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return strings.TrimSpace(string(runes[:max]))
}

// isContentFreeRender reports whether text carries no real content — only
// elision markers and artifact bookkeeping lines. Such a render spends
// tokens on zero signal; callers treat it as equivalent to empty.
func isContentFreeRender(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isRenderMarkerLine(line) {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.Trim(field, ".…") != "" {
				return false
			}
		}
	}
	return true
}

func isRenderMarkerLine(line string) bool {
	if line == "[full output saved]" {
		return true
	}
	if !strings.HasSuffix(line, "]") {
		return false
	}
	for _, prefix := range []string{"[tee: ", "[recovery: ", "[full output: "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
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
