package engine

import (
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/rules"
)

type Engine struct {
	config             config.Config
	paths              config.Paths
	history            *history.Store
	budgetAdapter      BudgetAdapter
	profiles           []Profile
	projectProfiles    []Profile
	builtinProfiles    []Profile
	projectPreferences []rules.Preference
}

func New(cfg config.Config, paths config.Paths, store *history.Store, profiles []Profile) *Engine {
	projectProfiles := annotateProfilesCapabilities(compileRuleProfiles(cfg))
	builtinProfiles := annotateProfilesCapabilities(annotateProfilesSource(profiles, SourceBuiltin))
	var budgetAdapter BudgetAdapter
	if cfg.Advanced.AdaptiveBudgets {
		budgetAdapter = NewHistoryBudgetAdapter(store)
	}
	return &Engine{
		config:             cfg,
		paths:              paths,
		history:            store,
		budgetAdapter:      budgetAdapter,
		profiles:           mergeProfiles(projectProfiles, builtinProfiles),
		projectProfiles:    projectProfiles,
		builtinProfiles:    builtinProfiles,
		projectPreferences: append([]rules.Preference(nil), cfg.ProjectRules.Preferences...),
	}
}

func (e *Engine) SetBudgetAdapter(adapter BudgetAdapter) {
	e.budgetAdapter = adapter
}

func shouldApplyBypass(profile Profile, decision FastPathDecision) bool {
	if !decision.BypassCompression {
		return false
	}
	if profile.StreamRender == nil {
		return false
	}
	return shouldBypassForDecisionMode(profile.Capabilities.FastPathBypass, decision)
}

func allowsHighConfidenceBypass(name string) bool {
	switch name {
	case "git-ls-files",
		"git-status",
		"grep",
		"path-find",
		"ripgrep-files",
		"ripgrep-files-with-matches":
		return true
	default:
		return false
	}
}

func bypassReason(decision FastPathDecision) string {
	if !decision.BypassCompression {
		return ""
	}
	return decision.Reason
}

func bytesForFastPath(profile Profile, result runResult) int {
	switch profile.StreamPreference {
	case StreamStdoutOnly, StreamStdoutFirst:
		return result.stdoutBytes
	case StreamStderrOnly, StreamStderrFirst:
		return result.stderrBytes
	default:
		return result.stdoutBytes + result.stderrBytes
	}
}

func shouldUseFailureEscape(profile Profile, exitCode int, passthrough bool, fallbackUsed bool) bool {
	if passthrough || exitCode == 0 || !fallbackUsed {
		return false
	}
	return profile.Capabilities.AllowFailureEscape
}
