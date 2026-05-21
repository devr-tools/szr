package engine

import (
	"strings"

	"szr/internal/config"
	"szr/internal/filters"
	"szr/internal/rules"
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

func matchRule(match rules.Match, inv Invocation) bool {
	if len(match.CommandPrefix) > 0 && !hasPrefix(inv.Command, match.CommandPrefix) {
		return false
	}
	if len(match.DisplayPrefix) > 0 && !hasPrefix(inv.Display, match.DisplayPrefix) {
		return false
	}

	args := invocationArgs(inv)
	if len(match.AllArgs) > 0 && !containsAll(args, match.AllArgs) {
		return false
	}
	if len(match.AnyArgs) > 0 && !containsAnyValue(args, match.AnyArgs) {
		return false
	}
	if len(match.ExcludeArgs) > 0 && containsAnyValue(args, match.ExcludeArgs) {
		return false
	}
	if len(match.CwdContains) > 0 && !containsAllSubstrings(inv.Cwd, match.CwdContains) {
		return false
	}
	return true
}

func rewriteRule(rewrite rules.Rewrite, inv Invocation) []string {
	mode := rewrite.Mode
	if mode == "" {
		mode = "append"
	}
	if len(rewrite.Args) == 0 {
		return inv.Command
	}
	if containsAnyValue(invocationArgs(inv), rewrite.SkipIfHasAny) {
		return inv.Command
	}

	switch mode {
	case "replace":
		return append([]string(nil), rewrite.Args...)
	default:
		command := append([]string(nil), inv.Command...)
		command = append(command, rewrite.Args...)
		return command
	}
}

func renderRule(render rules.Render, exec Execution, defaultMaxLines int) string {
	mode := render.Mode
	if mode == "" {
		mode = "compact"
	}
	maxLines := render.MaxLines
	if maxLines == 0 {
		maxLines = defaultMaxLines
	}

	combined := combineStreams(exec.Stdout, exec.Stderr)
	switch mode {
	case "failure":
		return filters.SummarizeGenericFailure(combined, maxLines)
	case "passthrough":
		return combined
	default:
		return filters.CompactLines(combined, maxLines)
	}
}

func explainRule(rule rules.Profile) []string {
	lines := make([]string, 0, len(rule.Explain)+2)
	if len(rule.Match.CommandPrefix) > 0 {
		lines = append(lines, "Matches command prefix `"+strings.Join(rule.Match.CommandPrefix, " ")+"`.")
	}
	if len(rule.Match.DisplayPrefix) > 0 {
		lines = append(lines, "Matches display prefix `"+strings.Join(rule.Match.DisplayPrefix, " ")+"`.")
	}
	if len(rule.Match.CwdContains) > 0 {
		lines = append(lines, "Matches cwd containing `"+strings.Join(rule.Match.CwdContains, "`, `")+"`.")
	}
	if len(rule.Explain) > 0 {
		lines = append(lines, rule.Explain...)
	}
	return lines
}

func hasPrefix(values []string, prefix []string) bool {
	if len(values) < len(prefix) {
		return false
	}
	for i, value := range prefix {
		if values[i] != value {
			return false
		}
	}
	return true
}

func containsAll(values []string, needles []string) bool {
	for _, needle := range needles {
		if !containsValue(values, needle) {
			return false
		}
	}
	return true
}

func containsAnyValue(values []string, needles []string) bool {
	for _, needle := range needles {
		if containsValue(values, needle) {
			return true
		}
	}
	return false
}

func containsValue(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func containsAllSubstrings(value string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

func invocationArgs(inv Invocation) []string {
	if len(inv.Command) <= 1 {
		return nil
	}
	return inv.Command[1:]
}
