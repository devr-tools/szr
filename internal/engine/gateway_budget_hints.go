package engine

import (
	"time"

	"github.com/devr-tools/szr/internal/budgethints"
	"github.com/devr-tools/szr/internal/history"
)

const BudgetAdaptationSourceGateway = "gateway"

// GatewayBudgetHintAdapter applies only prevalidated, locally stored gateway
// hints. It never reads the network, and every rejection falls back to the
// existing budget unchanged.
type GatewayBudgetHintAdapter struct {
	store    *budgethints.Store
	outcomes *budgethints.OutcomeStore
	opts     GatewayBudgetHintAdapterOptions
}

type GatewayBudgetHintAdapterOptions struct {
	Now               func() time.Time
	MinSamples        int
	MaxTightenPercent int
	MaxLoosenPercent  int
	MinDeltaLines     int
	MinDeltaBytes     int
	MinDeltaTokens    int
	OutcomeStore      *budgethints.OutcomeStore
}

func NewGatewayBudgetHintAdapter(store *budgethints.Store) *GatewayBudgetHintAdapter {
	return NewGatewayBudgetHintAdapterWithOptions(store, GatewayBudgetHintAdapterOptions{})
}

func NewGatewayBudgetHintAdapterWithOptions(store *budgethints.Store, opts GatewayBudgetHintAdapterOptions) *GatewayBudgetHintAdapter {
	if store == nil {
		return nil
	}
	return &GatewayBudgetHintAdapter{store: store, outcomes: opts.OutcomeStore, opts: normalizeGatewayBudgetHintAdapterOptions(opts)}
}

func normalizeGatewayBudgetHintAdapterOptions(opts GatewayBudgetHintAdapterOptions) GatewayBudgetHintAdapterOptions {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.MinSamples <= 0 {
		opts.MinSamples = 20
	}
	if opts.MaxTightenPercent <= 0 {
		opts.MaxTightenPercent = 10
	}
	if opts.MaxLoosenPercent <= 0 {
		opts.MaxLoosenPercent = 15
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

//nolint:maintidx // Every gateway-hint rejection is kept visible and fail-closed.
func (a *GatewayBudgetHintAdapter) AdaptBudget(profile Profile, inv Invocation, budget OutputBudget) (OutputBudget, *BudgetAdaptation) {
	if a == nil || a.store == nil || profile.Name == "" {
		return budget, nil
	}
	fingerprint := invocationFingerprint(inv)
	hint, err := a.store.Lookup(profile.Name, fingerprint, a.opts.Now())
	if err != nil || hint == nil || hint.Samples < a.opts.MinSamples {
		return budget, nil
	}
	if a.outcomes != nil && a.outcomes.ShouldRollback(*hint, a.opts.Now()) {
		return budget, nil
	}
	suggested := outputBudgetFromGatewayHint(budget, hint.Suggested)
	suggestion := history.BudgetSuggestion{Direction: history.BudgetSuggestionDirection(hint.Direction), Samples: hint.Samples}
	opts := HistoryBudgetAdapterOptions{
		MaxTightenPercent: a.opts.MaxTightenPercent,
		MaxLoosenPercent:  a.opts.MaxLoosenPercent,
		// A gateway hint never gets the exceptional fallback-heavy loosening
		// allowance reserved for locally observed history evidence.
		MaxFallbackHeavyLoosenPercent: a.opts.MaxLoosenPercent,
		MinDeltaLines:                 a.opts.MinDeltaLines,
		MinDeltaBytes:                 a.opts.MinDeltaBytes,
		MinDeltaTokens:                a.opts.MinDeltaTokens,
	}
	applied, ok := adaptBudgetConservatively(budget, suggested, suggestion, normalizeHistoryBudgetAdapterOptions(opts))
	if !ok {
		return budget, nil
	}
	return applied, &BudgetAdaptation{
		Source:      BudgetAdaptationSourceGateway,
		Fingerprint: fingerprint,
		Direction:   string(hint.Direction),
		Reason:      "gateway_hint",
		Confidence:  "gateway",
		Samples:     hint.Samples,
		Suggested:   suggested,
		Applied:     applied,
	}
}

// RecordOutcome is called after execution completes. Its errors are ignored
// by the engine because feedback must never change a command's result.
func (a *GatewayBudgetHintAdapter) RecordOutcome(adaptation *BudgetAdaptation, profile string, fallback bool, verifierRepairs int) {
	if a == nil || a.outcomes == nil || adaptation == nil || adaptation.Source != BudgetAdaptationSourceGateway {
		return
	}
	hint, err := a.store.Lookup(profile, adaptation.Fingerprint, a.opts.Now())
	if err != nil || hint == nil {
		return
	}
	_ = a.outcomes.Append(budgethints.Outcome{
		At: a.opts.Now(), Profile: hint.Profile, Fingerprint: hint.Fingerprint, ExpiresAt: hint.ExpiresAt,
		Fallback: fallback, Repair: verifierRepairs > 0,
	})
}

func outputBudgetFromGatewayHint(base OutputBudget, target budgethints.Target) OutputBudget {
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
