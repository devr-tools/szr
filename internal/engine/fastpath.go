package engine

import "time"

const (
	defaultTinyOutputBypassBytes  = 192
	defaultTinyOutputBypassTokens = 48
)

type FastPathDecision struct {
	BypassCompression bool
	Reason            string
	WarnLatency       bool
}

func ResolveBudget(profile Profile, fallbackLines int) OutputBudget {
	budget := profile.Budget
	if budget.MaxLines <= 0 {
		budget.MaxLines = fallbackLines
	}
	if budget.MaxBytes <= 0 && budget.MaxLines > 0 {
		budget.MaxBytes = budget.MaxLines * 160
	}
	return budget
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
