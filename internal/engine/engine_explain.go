package engine

func (e *Engine) Profiles() []Profile {
	return append([]Profile(nil), e.profiles...)
}

func (e *Engine) Explain(inv Invocation) Profile {
	preparedInv, _ := e.prepareInvocation(inv)
	return e.match(preparedInv)
}

func (e *Engine) ExplainBudget(inv Invocation) (OutputBudget, *BudgetAdaptation) {
	preparedInv, _ := e.prepareInvocation(inv)
	profile := e.match(preparedInv)
	return ResolveBudgetWithAdapter(profile, preparedInv, e.config.MaxPreviewLines, e.budgetAdapter)
}

func (e *Engine) ExplainDecisions(inv Invocation) []ExplainDecision {
	preparedInv, _ := e.prepareInvocation(inv)
	selected := e.match(preparedInv)
	decisions := make([]ExplainDecision, 0, len(e.projectProfiles)+len(e.builtinProfiles))

	for _, profile := range e.projectProfiles {
		if profile.Match != nil && profile.Match(preparedInv) {
			decisions = append(decisions, explainDecision(profile, profile.Name == selected.Name && profile.Source == selected.Source))
		}
	}
	for _, profile := range e.builtinProfiles {
		if profile.Match != nil && profile.Match(preparedInv) {
			decisions = append(decisions, explainDecision(profile, profile.Name == selected.Name && profile.Source == selected.Source))
		}
	}
	if len(decisions) == 0 {
		return []ExplainDecision{explainDecision(selected, true)}
	}
	return decisions
}

func (e *Engine) ExplainPreferences(inv Invocation) (Invocation, []PreferenceDecision) {
	return e.prepareInvocation(inv)
}

func (e *Engine) prepareInvocation(inv Invocation) (Invocation, []PreferenceDecision) {
	effective := inv
	effective.Command = append([]string(nil), inv.Command...)
	effective.Display = append([]string(nil), inv.Display...)
	effective.Advanced = e.config.Advanced

	decisions := make([]PreferenceDecision, 0, len(e.projectPreferences))
	for _, preference := range e.projectPreferences {
		if !matchRule(preference.Match, effective) {
			continue
		}
		rewritten := rewriteRule(preference.Rewrite, effective)
		applied := !sameStrings(effective.Command, rewritten)
		effective.Command = rewritten
		decisions = append(decisions, PreferenceDecision{
			Name:             preference.Name,
			Description:      preference.Description,
			Source:           SourcePreference,
			Applied:          applied,
			EffectiveCommand: append([]string(nil), effective.Command...),
			Explain:          preferenceExplainLines(preference),
		})
	}
	return classifyInvocation(effective), decisions
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
