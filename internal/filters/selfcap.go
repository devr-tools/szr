package filters

import (
	"fmt"
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

// The engine compression contract caps large renders at one fifth of the raw
// token cost (with a usable floor) and at the profile token budget. A filter
// that knows which lines matter can self-cap to that predicted allowance so
// the generic downstream token capper never has to pick survivors for it —
// the filter's tiering (failure names before assertion details before
// summaries) beats a density heuristic every time. The constants mirror the
// contract; see the git diff reducer's verbatimTokenAllowance for the same
// prediction on the streaming byte-count signal.
const (
	// selfCapContractRawTokens mirrors the contract's raw-token arming
	// threshold: below it the contract stays disarmed and no self-cap is
	// needed.
	selfCapContractRawTokens = 200
	// selfCapMinTokens mirrors the contract's usable floor.
	selfCapMinTokens = 48
	// selfCapSuffixReserve leaves room for the recovery/tee suffix the
	// display finalizer appends inside the same contract allowance.
	selfCapSuffixReserve = 16
	// selfCapTokensPerLine mirrors the profile budget's tokens-per-line
	// derivation (see profilekit.OutputBudget).
	selfCapTokensPerLine = 32
	// selfCapRetainedDen is the contract's retained fraction denominator.
	selfCapRetainedDen = 5
)

// PredictedTokenAllowance predicts the engine compression contract's
// retained-token budget for a summary of rawInput rendered under a profile
// budget of maxLines lines. Returns 0 when the contract is predicted to stay
// disarmed (small raw output), meaning the render needs no self-cap. The
// prediction uses the same lexical token estimate as the contract itself, so
// on the batch render path it matches exactly; on streaming paths the input
// the filter sees is ANSI-stripped and therefore never larger than the raw
// stream the contract measures, keeping the prediction conservative.
func PredictedTokenAllowance(rawInput string, maxLines int) int {
	rawTokens := history.EstimateTokens(rawInput)
	if rawTokens < selfCapContractRawTokens {
		return 0
	}
	allowed := (rawTokens + selfCapRetainedDen - 1) / selfCapRetainedDen
	if maxLines > 0 && maxLines*selfCapTokensPerLine < allowed {
		allowed = maxLines * selfCapTokensPerLine
	}
	if allowed < selfCapMinTokens {
		allowed = selfCapMinTokens
	}
	return allowed - selfCapSuffixReserve
}

// LineTokenCost prices one rendered line, including its newline, so a
// per-line sum is a safe upper bound for the whole-render token estimate the
// compression contract measures.
func LineTokenCost(line string) int {
	return history.EstimateTokens(line + "\n")
}

// PriorityLine is one candidate render line with its selection tier. Lower
// tiers are granted budget first regardless of position, so every tier-N
// line outranks every tier-N+1 line when the budget cannot hold both. Tier 0
// is the caller's irreducible failure inventory (failing test names,
// diagnostic headers): those lines bypass the token cap entirely, because a
// render that silently loses a failure is wrong no matter how cheap it is —
// the contract's own usable floor absorbs the mild overshoot.
type PriorityLine struct {
	Text string
	Tier int
}

// FitPriorityLines selects candidate lines within a line budget and a token
// budget. Budget is granted tier by tier (position order within a tier) and
// the survivors are returned in their original candidate order together with
// the number of omitted lines. A tokenBudget <= 0 means unlimited tokens and
// a maxLines <= 0 means unlimited lines. The first candidate of the
// most-important tier always survives, so the render is never content-free.
func FitPriorityLines(candidates []PriorityLine, maxLines, tokenBudget int) ([]string, int) {
	selected, omitted, _ := fitPriorityLines(candidates, maxLines, tokenBudget)
	return selected, omitted
}

// FitPriorityLinesWithMarker fits candidates like FitPriorityLines and
// appends a "... +N more lines" marker when lines were omitted — but only
// when the marker itself still fits the leftover token budget, so noting an
// omission can never push a self-capped render past the contract allowance.
func FitPriorityLinesWithMarker(candidates []PriorityLine, maxLines, tokenBudget int) string {
	selected, omitted, leftover := fitPriorityLines(candidates, maxLines, tokenBudget)
	text := strings.Join(selected, "\n")
	if omitted <= 0 {
		return text
	}
	marker := fmt.Sprintf("... +%d more lines", omitted)
	if tokenBudget > 0 && LineTokenCost(marker) > leftover {
		return text
	}
	return text + "\n" + marker
}

func fitPriorityLines(candidates []PriorityLine, maxLines, tokenBudget int) ([]string, int, int) {
	kept, keptCount, remaining := selectPriorityLines(candidates, maxLines, tokenBudget)
	out := make([]string, 0, keptCount)
	for i, candidate := range candidates {
		if kept[i] {
			out = append(out, candidate.Text)
		}
	}
	return out, len(candidates) - keptCount, remaining
}

// selectPriorityLines marks the candidates that survive the line and token
// budgets, most-important tier first, and returns the kept flags, the kept
// count, and the unspent token budget.
func selectPriorityLines(candidates []PriorityLine, maxLines, tokenBudget int) ([]bool, int, int) {
	kept := make([]bool, len(candidates))
	keptCount := 0
	remaining := tokenBudget
	for _, idx := range priorityLineOrder(candidates) {
		if maxLines > 0 && keptCount >= maxLines {
			break
		}
		cost := LineTokenCost(candidates[idx].Text)
		if tokenBudget > 0 && cost > remaining && keptCount > 0 && candidates[idx].Tier > 0 {
			continue
		}
		kept[idx] = true
		keptCount++
		remaining -= cost
	}
	return kept, keptCount, remaining
}

// priorityLineOrder returns candidate indexes ordered most-important-first:
// ascending tier, original position within a tier.
func priorityLineOrder(candidates []PriorityLine) []int {
	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return candidates[order[a]].Tier < candidates[order[b]].Tier
	})
	return order
}
