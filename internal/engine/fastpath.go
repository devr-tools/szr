package engine

import (
	"strings"
	"time"
)

const (
	defaultTinyOutputBypassBytes  = 192
	defaultTinyOutputBypassTokens = 48
)

type FastPathDecision struct {
	BypassCompression bool
	Reason            string
	WarnLatency       bool
}

func ResolveBudget(profile Profile, inv Invocation, fallbackLines int) OutputBudget {
	budget := profile.Budget
	if budget.MaxLines <= 0 {
		budget.MaxLines = fallbackLines
	}
	if budget.MaxLines <= 0 {
		budget.MaxLines = 12
	}
	if budget.MaxBytes <= 0 && budget.MaxLines > 0 {
		budget.MaxBytes = budget.MaxLines * 160
	}
	if budget.MaxTokens <= 0 && budget.MaxLines > 0 {
		budget.MaxTokens = budget.MaxLines * 32
	}

	budget = tuneBudgetByProfile(profile, budget)
	if inv.UltraCompact {
		budget = scaleBudget(budget, 3, 5)
	}
	switch {
	case inv.Verbose >= 2:
		budget = scaleBudget(budget, 3, 2)
	case inv.Verbose >= 1:
		budget = scaleBudget(budget, 5, 4)
	}
	switch profile.Confidence {
	case ConfidenceLow:
		budget = scaleBudget(budget, 5, 4)
	case ConfidenceMedium:
		budget = scaleBudget(budget, 9, 8)
	}

	budget.MaxLines = clampInt(budget.MaxLines, 3, 40)
	if budget.MaxBytes <= 0 {
		budget.MaxBytes = budget.MaxLines * 160
	}
	if budget.MaxTokens <= 0 {
		budget.MaxTokens = budget.MaxLines * 32
	}
	return budget
}

func ExpandBudgetForFailureEscape(budget OutputBudget, inv Invocation) OutputBudget {
	if inv.UltraCompact {
		return scaleBudget(budget, 5, 4)
	}
	return scaleBudget(budget, 3, 2)
}

func DecideFastPath(profile Profile, rawBytes, rawTokens int, duration time.Duration, exitCode int) FastPathDecision {
	decision := FastPathDecision{}
	if profile.LatencyBudget > 0 && duration > profile.LatencyBudget {
		decision.WarnLatency = true
	}
	if exitCode != 0 {
		return decision
	}
	if profile.StreamPreference == StreamStderrOnly && rawBytes == 0 {
		decision.BypassCompression = true
		decision.Reason = "stderr-only profile with empty stderr payload"
		return decision
	}
	if rawBytes <= defaultTinyOutputBypassBytes && rawTokens <= defaultTinyOutputBypassTokens {
		decision.BypassCompression = true
		decision.Reason = "tiny output fast path"
	}
	return decision
}

func tuneBudgetByProfile(profile Profile, budget OutputBudget) OutputBudget {
	name := profile.Name
	switch {
	case strings.Contains(name, "log"):
		if budget.MaxLines < 12 {
			budget.MaxLines = 12
		}
	case name == "ripgrep", name == "generic-summary", name == "git-log", name == "git-status":
		budget = scaleBudget(budget, 4, 5)
	case strings.Contains(name, "test"), strings.Contains(name, "build"), name == "pytest", name == "ctest":
		if budget.MaxLines < 10 {
			budget.MaxLines = 10
		}
	}
	return budget
}

func scaleBudget(budget OutputBudget, num, den int) OutputBudget {
	if num <= 0 || den <= 0 {
		return budget
	}
	budget.MaxLines = scaleIntCeil(budget.MaxLines, num, den)
	budget.MaxBytes = scaleIntCeil(budget.MaxBytes, num, den)
	budget.MaxTokens = scaleIntCeil(budget.MaxTokens, num, den)
	return budget
}

func scaleIntCeil(value, num, den int) int {
	if value <= 0 {
		return value
	}
	return (value*num + den - 1) / den
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
