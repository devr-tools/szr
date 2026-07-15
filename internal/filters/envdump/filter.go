package envdump

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

const (
	maxEnvLineWidth  = 160
	maxShownPathDirs = 4
	maxRawValueLen   = 32
)

var secretNameMarkers = []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL"}

var diagnosticNames = map[string]bool{
	"HOME": true, "SHELL": true, "PWD": true, "OLDPWD": true,
	"USER": true, "LOGNAME": true, "LANG": true, "LC_ALL": true,
	"TERM": true, "TMPDIR": true, "EDITOR": true, "VISUAL": true,
	"PAGER": true,
}

type envVar struct {
	name  string
	value string
}

type envSummaryResult struct {
	text         string
	omittedCount int
}

func SummarizeEnvDump(input string, maxLines int) string {
	return summarizeEnvDumpResult(input, maxLines).text
}

func EnvDumpRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeEnvDumpResult(input, maxLines)
	if result.omittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.omittedCount))
}

func summarizeEnvDumpResult(input string, maxLines int) envSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}
	clean := shared.StripANSI(input)
	vars := parseEnvVars(clean)
	if len(vars) == 0 {
		return envSummaryResult{text: shared.CompactLines(strings.TrimSpace(clean), maxLines)}
	}
	out := append([]string{fmt.Sprintf("env: %d vars", len(vars))}, envDetailLines(vars)...)
	result := envSummaryResult{text: shared.JoinLimitedLines(out, maxLines)}
	if len(out) > maxLines {
		result.omittedCount = len(out) - maxLines
	}
	return result
}

func parseEnvVars(input string) []envVar {
	lines := shared.NonEmptyLines(input)
	out := make([]envVar, 0, len(lines))
	for _, line := range lines {
		name, value, ok := strings.Cut(line, "=")
		if !ok || name == "" || strings.ContainsAny(name, " \t") {
			continue
		}
		out = append(out, envVar{name: name, value: value})
	}
	return out
}

// envDetailLines orders variables by diagnostic value: PATH and common shell
// context stay readable up front, secret-looking values surface as redaction
// markers, and everything else folds into prefix groups.
func envDetailLines(vars []envVar) []string {
	out := []string{}
	diagnostic := []string{}
	secret := []string{}
	rest := []envVar{}
	for _, v := range vars {
		switch {
		case v.name == "PATH":
			out = append(out, summarizePathEntries(v))
		case diagnosticNames[v.name]:
			diagnostic = append(diagnostic, shared.Clip(v.name+"="+v.value, maxEnvLineWidth))
		case isSecretName(v.name):
			secret = append(secret, formatEnvValue(v))
		default:
			rest = append(rest, v)
		}
	}
	out = append(out, packLines(diagnostic)...)
	out = append(out, packLines(secret)...)
	return append(out, groupedEnvLines(rest)...)
}

func summarizePathEntries(v envVar) string {
	entries := strings.Split(v.value, ":")
	shown := entries
	if len(shown) > maxShownPathDirs {
		shown = shown[:maxShownPathDirs]
	}
	line := fmt.Sprintf("%s: %d entries: %s", v.name, len(entries), strings.Join(shown, " "))
	if len(entries) > len(shown) {
		line += fmt.Sprintf(" (+%d more)", len(entries)-len(shown))
	}
	return shared.Clip(line, maxEnvLineWidth)
}

func isSecretName(name string) bool {
	upper := strings.ToUpper(name)
	for _, marker := range secretNameMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return false
}

func formatEnvValue(v envVar) string {
	if isSecretName(v.name) {
		return fmt.Sprintf("%s=<redacted len=%d>", v.name, len(v.value))
	}
	if len(v.value) > maxRawValueLen {
		return fmt.Sprintf("%s=<len %d>", v.name, len(v.value))
	}
	return v.name + "=" + v.value
}

// groupedEnvLines renders one line per prefix family (two or more members)
// and packs the leftovers into shared width-bounded lines.
func groupedEnvLines(vars []envVar) []string {
	groups, order := collectEnvGroups(vars)
	out := []string{}
	loose := []string{}
	for _, key := range order {
		members := groups[key]
		if key == "" || len(members) < 2 {
			loose = append(loose, members...)
			continue
		}
		out = append(out, formatEnvGroup(key, members))
	}
	return append(out, packLines(loose)...)
}

func collectEnvGroups(vars []envVar) (map[string][]string, []string) {
	groups := map[string][]string{}
	order := []string{}
	for _, v := range vars {
		key := envPrefix(v.name)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], formatEnvValue(v))
	}
	return groups, order
}

func formatEnvGroup(key string, members []string) string {
	line := fmt.Sprintf("%s_* (%d): %s", key, len(members), strings.Join(members, " "))
	return shared.Clip(line, maxEnvLineWidth)
}

func envPrefix(name string) string {
	if idx := strings.Index(name, "_"); idx > 0 {
		return name[:idx]
	}
	return ""
}

func packLines(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := []string{}
	current := ""
	for _, item := range items {
		if current != "" && len(current)+1+len(item) > maxEnvLineWidth {
			out = append(out, current)
			current = ""
		}
		current = joinPacked(current, item)
	}
	return append(out, shared.Clip(current, maxEnvLineWidth))
}

func joinPacked(current, item string) string {
	if current == "" {
		return item
	}
	return current + " " + item
}
