package engine

// FirstBudgetAdapter uses the first adapter that produces a conservative
// adjustment. This lets locally stored, signed gateway advice participate
// without overriding the existing history adapter when it has no advice.
type FirstBudgetAdapter struct{ adapters []BudgetAdapter }

func NewFirstBudgetAdapter(adapters ...BudgetAdapter) *FirstBudgetAdapter {
	filtered := make([]BudgetAdapter, 0, len(adapters))
	for _, adapter := range adapters {
		if adapter != nil {
			filtered = append(filtered, adapter)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return &FirstBudgetAdapter{adapters: filtered}
}

func (a *FirstBudgetAdapter) AdaptBudget(profile Profile, inv Invocation, budget OutputBudget) (OutputBudget, *BudgetAdaptation) {
	if a == nil {
		return budget, nil
	}
	for _, adapter := range a.adapters {
		adapted, adaptation := adapter.AdaptBudget(profile, inv, budget)
		if adaptation != nil {
			return adapted, adaptation
		}
	}
	return budget, nil
}

func (a *FirstBudgetAdapter) RecordOutcome(adaptation *BudgetAdaptation, profile string, fallback bool, verifierRepairs int) {
	if a == nil {
		return
	}
	for _, adapter := range a.adapters {
		if recorder, ok := adapter.(interface {
			RecordOutcome(*BudgetAdaptation, string, bool, int)
		}); ok {
			recorder.RecordOutcome(adaptation, profile, fallback, verifierRepairs)
		}
	}
}
