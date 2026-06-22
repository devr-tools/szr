package engine

import (
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

type ultraCompactCandidate struct {
	text       string
	score      int
	lineIndex  int
	fromRender bool
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
