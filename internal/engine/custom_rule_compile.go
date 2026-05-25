package engine

import (
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/rules"
)

func compileRuleProfiles(cfg config.Config) []Profile {
	if len(cfg.ProjectRules.Profiles) == 0 {
		return nil
	}

	compiled := make([]Profile, 0, len(cfg.ProjectRules.Profiles))
	for _, rule := range cfg.ProjectRules.Profiles {
		rule := rule
		compiled = append(compiled, Profile{
			Name:        rule.Name,
			Description: rule.Description,
			Source:      SourceProject,
			Match: func(inv Invocation) bool {
				return matchRule(rule.Match, inv)
			},
			Prepare: func(inv Invocation) []string {
				return rewriteRule(rule.Rewrite, inv)
			},
			Render: func(_ Invocation, exec Execution) string {
				return renderRule(rule.Render, exec, cfg.MaxPreviewLines)
			},
			Explain: explainRule(rule),
		})
	}
	return compiled
}

func mergeProfiles(custom []Profile, builtins []Profile) []Profile {
	if len(custom) == 0 {
		return append([]Profile(nil), builtins...)
	}

	merged := make([]Profile, 0, len(custom)+len(builtins))
	seen := make(map[string]struct{}, len(custom)+len(builtins))

	for _, profile := range custom {
		merged = append(merged, profile)
		seen[profile.Name] = struct{}{}
	}
	for _, profile := range builtins {
		if _, exists := seen[profile.Name]; exists {
			continue
		}
		merged = append(merged, profile)
		seen[profile.Name] = struct{}{}
	}
	return merged
}

func annotateProfilesSource(profiles []Profile, source string) []Profile {
	annotated := make([]Profile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.Source == "" {
			profile.Source = source
		}
		annotated = append(annotated, profile)
	}
	return annotated
}

func explainDecision(profile Profile, selected bool) ExplainDecision {
	return ExplainDecision{
		Name:        profile.Name,
		Description: profile.Description,
		Source:      profile.Source,
		Selected:    selected,
		Explain:     append([]string(nil), profile.Explain...),
	}
}

func preferenceExplainLines(preference rules.Preference) []string {
	lines := make([]string, 0, len(preference.Explain)+3)
	if len(preference.Match.CommandPrefix) > 0 {
		lines = append(lines, "Matches command prefix `"+stringsJoin(preference.Match.CommandPrefix)+"`.")
	}
	if len(preference.Match.DisplayPrefix) > 0 {
		lines = append(lines, "Matches display prefix `"+stringsJoin(preference.Match.DisplayPrefix)+"`.")
	}
	if len(preference.Match.CwdContains) > 0 {
		lines = append(lines, "Matches cwd containing `"+joinQuoted(preference.Match.CwdContains)+"`.")
	}
	mode := preference.Rewrite.Mode
	if mode == "" {
		mode = "append"
	}
	lines = append(lines, "Applies preference rewrite mode `"+mode+"`.")
	if len(preference.Explain) > 0 {
		lines = append(lines, preference.Explain...)
	}
	return lines
}
