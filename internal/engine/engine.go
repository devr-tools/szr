package engine

import (
	"szr/internal/config"
	"szr/internal/history"
	"szr/internal/rules"
)

type Engine struct {
	config             config.Config
	paths              config.Paths
	history            *history.Store
	profiles           []Profile
	projectProfiles    []Profile
	builtinProfiles    []Profile
	projectPreferences []rules.Preference
}

func New(cfg config.Config, paths config.Paths, store *history.Store, profiles []Profile) *Engine {
	projectProfiles := compileRuleProfiles(cfg)
	builtinProfiles := annotateProfilesSource(profiles, SourceBuiltin)
	return &Engine{
		config:             cfg,
		paths:              paths,
		history:            store,
		profiles:           mergeProfiles(projectProfiles, builtinProfiles),
		projectProfiles:    projectProfiles,
		builtinProfiles:    builtinProfiles,
		projectPreferences: append([]rules.Preference(nil), cfg.ProjectRules.Preferences...),
	}
}

func shouldApplyBypass(profile Profile, decision FastPathDecision) bool {
	if !decision.BypassCompression {
		return false
	}
	if profile.Name == "passthrough" {
		return true
	}
	if profile.StreamRender == nil {
		return false
	}
	return profile.Confidence != ConfidenceHigh
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
	if profile.Name == "passthrough" {
		return false
	}
	return profile.Confidence != ConfidenceHigh
}
