package engine

import (
	"strings"

	"szr/internal/rules"
)

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
