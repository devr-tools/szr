package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/rules"
)

func explainRule(rule rules.Profile) []string {
	lines := make([]string, 0, len(rule.Explain)+2)
	if len(rule.Match.CommandPrefix) > 0 {
		lines = append(lines, "Matches command prefix `"+stringsJoin(rule.Match.CommandPrefix)+"`.")
	}
	if len(rule.Match.DisplayPrefix) > 0 {
		lines = append(lines, "Matches display prefix `"+stringsJoin(rule.Match.DisplayPrefix)+"`.")
	}
	if len(rule.Match.CwdContains) > 0 {
		lines = append(lines, "Matches cwd containing `"+joinQuoted(rule.Match.CwdContains)+"`.")
	}
	if len(rule.Explain) > 0 {
		lines = append(lines, rule.Explain...)
	}
	return lines
}

func stringsJoin(values []string) string {
	return strings.Join(values, " ")
}

func joinQuoted(values []string) string {
	return strings.Join(values, "`, `")
}
