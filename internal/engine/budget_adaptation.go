package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const BudgetAdaptationSourceHistory = "history"

type BudgetAdapter interface {
	AdaptBudget(profile Profile, inv Invocation, budget OutputBudget) (OutputBudget, *BudgetAdaptation)
}

type BudgetAdaptation struct {
	Source      string
	Fingerprint string
	Direction   string
	Reason      string
	Confidence  string
	Samples     int
	Scale       float64
	Suggested   OutputBudget
	Applied     OutputBudget
}

type HistoryBudgetAdapterOptions struct {
	SuggestionOptions             history.BudgetSuggestionOptions
	MinConfidence                 string
	MaxTightenPercent             int
	MaxLoosenPercent              int
	MaxFallbackHeavyLoosenPercent int
	MinDeltaLines                 int
	MinDeltaBytes                 int
	MinDeltaTokens                int
}

type HistoryBudgetAdapter struct {
	store *history.Store
	opts  HistoryBudgetAdapterOptions
}

func NewHistoryBudgetAdapter(store *history.Store) *HistoryBudgetAdapter {
	if store == nil {
		return nil
	}
	return &HistoryBudgetAdapter{
		store: store,
		opts:  normalizeHistoryBudgetAdapterOptions(HistoryBudgetAdapterOptions{}),
	}
}

func NewHistoryBudgetAdapterWithOptions(store *history.Store, opts HistoryBudgetAdapterOptions) *HistoryBudgetAdapter {
	if store == nil {
		return nil
	}
	return &HistoryBudgetAdapter{
		store: store,
		opts:  normalizeHistoryBudgetAdapterOptions(opts),
	}
}

func (a *HistoryBudgetAdapter) AdaptBudget(profile Profile, inv Invocation, budget OutputBudget) (OutputBudget, *BudgetAdaptation) {
	if a == nil || a.store == nil {
		return budget, nil
	}
	fingerprint := invocationFingerprint(inv)
	if fingerprint == "" {
		return budget, nil
	}
	suggestion, err := a.store.FindBudgetSuggestion(fingerprint, a.opts.SuggestionOptions)
	if err != nil || suggestion == nil {
		return budget, nil
	}
	if suggestion.Profile != "" && profile.Name != "" && suggestion.Profile != profile.Name {
		return budget, nil
	}
	if confidenceRank(suggestion.Confidence) < confidenceRank(a.opts.MinConfidence) {
		return budget, nil
	}

	suggestedBudget := outputBudgetFromSuggestion(budget, suggestion.Suggested)
	appliedBudget, ok := adaptBudgetConservatively(budget, suggestedBudget, *suggestion, a.opts)
	if !ok {
		return budget, nil
	}
	return appliedBudget, &BudgetAdaptation{
		Source:      BudgetAdaptationSourceHistory,
		Fingerprint: suggestion.Fingerprint,
		Direction:   string(suggestion.Direction),
		Reason:      string(suggestion.Reason),
		Confidence:  suggestion.Confidence,
		Samples:     suggestion.Samples,
		Scale:       suggestion.Scale,
		Suggested:   suggestedBudget,
		Applied:     appliedBudget,
	}
}

func normalizeHistoryBudgetAdapterOptions(opts HistoryBudgetAdapterOptions) HistoryBudgetAdapterOptions {
	if opts.MinConfidence == "" {
		opts.MinConfidence = "medium"
	}
	if opts.MaxTightenPercent <= 0 {
		opts.MaxTightenPercent = 20
	}
	if opts.MaxLoosenPercent <= 0 {
		opts.MaxLoosenPercent = 25
	}
	if opts.MaxFallbackHeavyLoosenPercent <= 0 {
		opts.MaxFallbackHeavyLoosenPercent = 50
	}
	if opts.MinDeltaLines <= 0 {
		opts.MinDeltaLines = 2
	}
	if opts.MinDeltaBytes <= 0 {
		opts.MinDeltaBytes = 64
	}
	if opts.MinDeltaTokens <= 0 {
		opts.MinDeltaTokens = 12
	}
	return opts
}

func invocationFingerprint(inv Invocation) string {
	commandText := strings.Join(inv.Display, " ")
	if strings.TrimSpace(commandText) == "" {
		commandText = strings.Join(inv.Command, " ")
	}
	return history.Fingerprint(commandText)
}

func outputBudgetFromSuggestion(base OutputBudget, target history.BudgetTarget) OutputBudget {
	suggested := base
	if target.MaxLines > 0 {
		suggested.MaxLines = target.MaxLines
	}
	if target.MaxBytes > 0 {
		suggested.MaxBytes = target.MaxBytes
	}
	if target.MaxTokens > 0 {
		suggested.MaxTokens = target.MaxTokens
	}
	return finalizeResolvedBudget(suggested)
}

func adaptBudgetConservatively(
	base OutputBudget,
	suggested OutputBudget,
	suggestion history.BudgetSuggestion,
	opts HistoryBudgetAdapterOptions,
) (OutputBudget, bool) {
	var adapted OutputBudget
	switch suggestion.Direction {
	case history.BudgetSuggestionTighten:
		tightest := scaleBudgetByPercent(base, 100-opts.MaxTightenPercent)
		adapted = maxBudget(suggested, tightest)
	case history.BudgetSuggestionLoosen:
		percent := 100 + opts.MaxLoosenPercent
		if suggestion.Reason == history.BudgetSuggestionFallbackHeavy && suggestion.Confidence == "high" {
			percent = 100 + opts.MaxFallbackHeavyLoosenPercent
		}
		loosest := scaleBudgetByPercent(base, percent)
		adapted = minBudget(suggested, loosest)
	default:
		return base, false
	}
	adapted = finalizeResolvedBudget(adapted)
	if !materialBudgetDelta(base, adapted, opts) {
		return base, false
	}
	if suggestion.Direction == history.BudgetSuggestionTighten && !budgetLessThan(adapted, base) {
		return base, false
	}
	if suggestion.Direction == history.BudgetSuggestionLoosen && !budgetGreaterThan(adapted, base) {
		return base, false
	}
	return adapted, true
}

func scaleBudgetByPercent(budget OutputBudget, percent int) OutputBudget {
	if percent <= 0 {
		return budget
	}
	return OutputBudget{
		MaxLines:    scaleIntCeil(budget.MaxLines, percent, 100),
		MaxBytes:    scaleIntCeil(budget.MaxBytes, percent, 100),
		MaxTokens:   scaleIntCeil(budget.MaxTokens, percent, 100),
		MinFailures: budget.MinFailures,
		MinAnchors:  budget.MinAnchors,
		MinHints:    budget.MinHints,
	}
}

func maxBudget(left, right OutputBudget) OutputBudget {
	return OutputBudget{
		MaxLines:    maxInt(left.MaxLines, right.MaxLines),
		MaxBytes:    maxInt(left.MaxBytes, right.MaxBytes),
		MaxTokens:   maxInt(left.MaxTokens, right.MaxTokens),
		MinFailures: left.MinFailures,
		MinAnchors:  left.MinAnchors,
		MinHints:    left.MinHints,
	}
}

func minBudget(left, right OutputBudget) OutputBudget {
	return OutputBudget{
		MaxLines:    minPositiveInt(left.MaxLines, right.MaxLines),
		MaxBytes:    minPositiveInt(left.MaxBytes, right.MaxBytes),
		MaxTokens:   minPositiveInt(left.MaxTokens, right.MaxTokens),
		MinFailures: left.MinFailures,
		MinAnchors:  left.MinAnchors,
		MinHints:    left.MinHints,
	}
}

func materialBudgetDelta(base, adapted OutputBudget, opts HistoryBudgetAdapterOptions) bool {
	if absInt(base.MaxLines-adapted.MaxLines) >= opts.MinDeltaLines {
		return true
	}
	if absInt(base.MaxBytes-adapted.MaxBytes) >= opts.MinDeltaBytes {
		return true
	}
	return absInt(base.MaxTokens-adapted.MaxTokens) >= opts.MinDeltaTokens
}

func budgetLessThan(left, right OutputBudget) bool {
	return left.MaxLines < right.MaxLines || left.MaxBytes < right.MaxBytes || left.MaxTokens < right.MaxTokens
}

func budgetGreaterThan(left, right OutputBudget) bool {
	return left.MaxLines > right.MaxLines || left.MaxBytes > right.MaxBytes || left.MaxTokens > right.MaxTokens
}

func confidenceRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func minPositiveInt(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
