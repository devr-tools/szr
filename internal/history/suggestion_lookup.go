package history

func FindBudgetSuggestion(records []Record, fingerprint string, opts BudgetSuggestionOptions) *BudgetSuggestion {
	if fingerprint == "" {
		return nil
	}
	suggestions := SuggestBudgets(records, opts)
	for i := range suggestions {
		if suggestions[i].Fingerprint != fingerprint {
			continue
		}
		suggestion := suggestions[i]
		return &suggestion
	}
	return nil
}
