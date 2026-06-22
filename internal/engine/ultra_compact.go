package engine

import (
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

const (
	ultraCompactSingleLineMaxTokens = 24
	ultraCompactSummaryMaxTokens    = 14
	ultraCompactDetailMaxTokens     = 24
)

type ultraCompactCandidate struct {
	text       string
	score      int
	lineIndex  int
	fromRender bool
}

func applyUltraCompactRender(inv Invocation, exec Execution, rendered string, rawCombined string) string {
	if !inv.UltraCompact {
		return rendered
	}
	lines, ok := normalizedUltraCompactLines(rendered)
	if !ok {
		return strings.TrimSpace(rendered)
	}
	return renderUltraCompactLines(lines, rawCombined, exec.ExitCode)
}

func compactNonEmptyLines(text string) []string {
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func ultraCompactDetailLines(renderedLines []string, rawCombined string, exitCode int) ([]string, int) {
	candidates := collectUltraCompactCandidates(renderedLines, rawCombined, exitCode)
	if len(candidates) == 0 {
		return nil, 0
	}
	selected, keptRendered := selectUltraCompactCandidates(candidates, ultraCompactMaxDetails(exitCode))
	if len(selected) == 0 {
		selected = append(selected, renderedLines[1])
		keptRendered = 1
	}
	return selected, keptRendered
}

func collectUltraCompactCandidates(renderedLines []string, rawCombined string, exitCode int) []ultraCompactCandidate {
	builder := newUltraCompactCandidateBuilder(renderedLines[0], len(renderedLines))
	builder.addRenderedLines(renderedLines, exitCode)
	builder.addRawFailureLines(renderedLines, rawCombined, exitCode)
	candidates := builder.candidates
	sort.SliceStable(candidates, compareUltraCompactCandidates(candidates))
	return candidates
}

type ultraCompactCandidateBuilder struct {
	headline   string
	baseIndex  int
	seen       map[string]struct{}
	candidates []ultraCompactCandidate
}

func newUltraCompactCandidateBuilder(headline string, capacity int) *ultraCompactCandidateBuilder {
	return &ultraCompactCandidateBuilder{
		headline:   headline,
		baseIndex:  capacity,
		seen:       map[string]struct{}{},
		candidates: make([]ultraCompactCandidate, 0, capacity),
	}
}

func (b *ultraCompactCandidateBuilder) addRenderedLines(renderedLines []string, exitCode int) {
	for i, line := range renderedLines[1:] {
		b.add(line, ultraCompactLineScore(line, i+1, exitCode), i+1, true)
	}
}

func (b *ultraCompactCandidateBuilder) addRawFailureLines(renderedLines []string, rawCombined string, exitCode int) {
	if exitCode == 0 || rawCombined == "" {
		return
	}
	for i, line := range compactNonEmptyLines(filters.InterestingErrorLines(rawCombined, 4)) {
		b.add(line, ultraCompactLineScore(line, len(renderedLines)+i, exitCode)+12, len(renderedLines)+i, false)
	}
}

func (b *ultraCompactCandidateBuilder) add(text string, score int, lineIndex int, fromRender bool) {
	text = strings.TrimSpace(text)
	if text == "" || text == b.headline {
		return
	}
	if _, ok := b.seen[text]; ok {
		return
	}
	b.seen[text] = struct{}{}
	b.candidates = append(b.candidates, ultraCompactCandidate{
		text:       text,
		score:      score,
		lineIndex:  lineIndex,
		fromRender: fromRender,
	})
}

func compareUltraCompactCandidates(candidates []ultraCompactCandidate) func(int, int) bool {
	return func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].fromRender != candidates[j].fromRender {
			return candidates[i].fromRender
		}
		return candidates[i].lineIndex < candidates[j].lineIndex
	}
}

func ultraCompactMaxDetails(exitCode int) int {
	if exitCode != 0 {
		return 3
	}
	return 2
}

func selectUltraCompactCandidates(candidates []ultraCompactCandidate, maxDetails int) ([]string, int) {
	selected := make([]string, 0, maxDetails)
	keptRendered := 0
	for _, candidate := range candidates {
		if candidate.score <= 0 {
			continue
		}
		selected = append(selected, candidate.text)
		if candidate.fromRender {
			keptRendered++
		}
		if len(selected) == maxDetails {
			break
		}
	}
	return selected, keptRendered
}

func ultraCompactLineScore(line string, lineIndex int, exitCode int) int {
	score := 0
	if lineIndex == 1 {
		score += 8
	}
	lower := strings.ToLower(line)
	score += ultraCompactSummaryPatternScore(line, lower)
	score += ultraCompactFailurePatternScore(lower, exitCode)
	for _, field := range strings.Fields(line) {
		score += compressionAnchorScore(field)
	}
	score += compressionContextScore(line)
	return score
}

func buildUltraCompactDetail(details []string, omitted int) string {
	parts := ultraCompactDetailParts(details, omitted)
	if len(parts) == 0 {
		return ""
	}
	return compressUltraCompactDetail(parts)
}

func normalizedUltraCompactLines(rendered string) ([]string, bool) {
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return nil, false
	}
	lines := compactNonEmptyLines(rendered)
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

func renderUltraCompactLines(lines []string, rawCombined string, exitCode int) string {
	if len(lines) == 1 {
		return hardCapTokens(lines[0], ultraCompactSingleLineMaxTokens)
	}
	summary := ultraCompactSummaryLine(lines[0])
	details, keptRendered := ultraCompactDetailLines(lines, rawCombined, exitCode)
	detail := buildUltraCompactDetail(details, len(lines)-1-keptRendered)
	if detail == "" {
		return summary
	}
	return summary + "\n" + detail
}

func ultraCompactSummaryLine(line string) string {
	summary := hardCapTokens(line, ultraCompactSummaryMaxTokens)
	if strings.TrimSpace(summary) == "" {
		return line
	}
	return summary
}

func ultraCompactSummaryPatternScore(line string, lower string) int {
	switch {
	case strings.HasPrefix(line, "[recovery:"), strings.HasPrefix(line, "[full output"):
		return -32
	case strings.Contains(lower, "matches across"),
		strings.Contains(lower, "files="),
		strings.HasPrefix(lower, "dirs:"),
		strings.HasPrefix(lower, "... +"):
		return 18
	case strings.HasPrefix(lower, "examples:"):
		return 10
	case strings.Contains(lower, "match") && strings.Contains(line, "("):
		return 14
	default:
		return 0
	}
}

func ultraCompactFailurePatternScore(lower string, exitCode int) int {
	if exitCode != 0 && (strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic")) {
		return 18
	}
	return 0
}

func ultraCompactDetailParts(details []string, omitted int) []string {
	parts := make([]string, 0, len(details)+1)
	for _, detail := range details {
		if trimmed := strings.TrimSpace(detail); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if omitted > 0 {
		parts = append(parts, "... +"+itoa(omitted)+" lines")
	}
	return parts
}

func compressUltraCompactDetail(parts []string) string {
	detail := strings.Join(parts, " | ")
	if history.EstimateTokens(detail) <= ultraCompactDetailMaxTokens {
		return detail
	}
	compressed := hardCapTokens(detail, ultraCompactDetailMaxTokens)
	if strings.TrimSpace(compressed) == "" {
		return parts[0]
	}
	return compressed
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return sign + string(buf[i:])
}
