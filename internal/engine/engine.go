package engine

import (
	"path/filepath"

	"github.com/devr-tools/szr/internal/budgethints"
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
	userProfiles       []Profile
	projectPreferences []rules.Preference
}

func New(cfg config.Config, paths config.Paths, store *history.Store, profiles []Profile) *Engine {
	projectProfiles := annotateProfilesCapabilities(compileRuleProfiles(cfg))
	builtinProfiles := annotateProfilesCapabilities(annotateProfilesSource(profiles, SourceBuiltin))
	// User filters register after builtins so builtins win match conflicts.
	userProfiles := annotateProfilesCapabilities(LoadUserFilterProfiles(
		defaultUserFilterSources(cfg, paths),
		cfg.MaxPreviewLines,
		profileNameSet(projectProfiles, builtinProfiles),
	))
	var adapters []BudgetAdapter
	if cfg.GatewayHints.Enabled {
		gateway := NewGatewayBudgetHintAdapterWithOptions(
			budgethints.New(filepath.Join(paths.DataDir, "gateway-budget-hints.json")),
			GatewayBudgetHintAdapterOptions{OutcomeStore: budgethints.NewOutcomeStore(filepath.Join(paths.DataDir, "gateway-budget-hint-outcomes.jsonl"))},
		)
		adapters = append(adapters, gateway)
	}
	if cfg.Advanced.AdaptiveBudgets {
		adapters = append(adapters, NewHistoryBudgetAdapter(store))
	}
	return &Engine{
		config:             cfg,
		paths:              paths,
		history:            store,
		budgetAdapter:      NewFirstBudgetAdapter(adapters...),
		profiles:           append(mergeProfiles(projectProfiles, builtinProfiles), userProfiles...),
		projectProfiles:    projectProfiles,
		builtinProfiles:    builtinProfiles,
		userProfiles:       userProfiles,
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
	if passthrough || !isFailureExit(profile, exitCode) || !fallbackUsed {
		return false
	}
	return profile.Capabilities.AllowFailureEscape
}
