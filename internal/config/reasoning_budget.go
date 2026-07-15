package config

import (
	"fmt"
	"strings"
)

const (
	ReasoningBudgetStandard   = "standard"
	ReasoningBudgetAgent      = "agent"
	ReasoningBudgetAggressive = "aggressive"
)

func NormalizeReasoningBudgetMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", ReasoningBudgetStandard:
		return ReasoningBudgetStandard, nil
	case ReasoningBudgetAgent, "loop", "agent-loop":
		return ReasoningBudgetAgent, nil
	case ReasoningBudgetAggressive, "tight", "spread":
		return ReasoningBudgetAggressive, nil
	default:
		return "", fmt.Errorf("invalid reasoning budget mode %q (want %q, %q, or %q)", value, ReasoningBudgetStandard, ReasoningBudgetAgent, ReasoningBudgetAggressive)
	}
}

func ResolveReasoningBudgetMode(value string) string {
	mode, err := NormalizeReasoningBudgetMode(value)
	if err != nil {
		return ReasoningBudgetStandard
	}
	return mode
}

func Normalize(cfg Config) (Config, error) {
	mode, err := NormalizeReasoningBudgetMode(cfg.ReasoningBudgetMode)
	if err != nil {
		return Config{}, err
	}
	cfg.ReasoningBudgetMode = mode
	if cfg.UpdateCheck.IntervalHours <= 0 {
		cfg.UpdateCheck.IntervalHours = Default().UpdateCheck.IntervalHours
	}
	if cfg.Advanced.SessionDedupWindowMinutes <= 0 {
		cfg.Advanced.SessionDedupWindowMinutes = DefaultSessionDedupWindowMinutes
	}
	if cfg.TeeMaxFileMB <= 0 {
		cfg.TeeMaxFileMB = DefaultTeeMaxFileMB
	}
	if cfg.TeeMaxDirFiles <= 0 {
		cfg.TeeMaxDirFiles = DefaultTeeMaxDirFiles
	}
	if cfg.TeeMaxDirMB <= 0 {
		cfg.TeeMaxDirMB = DefaultTeeMaxDirMB
	}
	if cfg.CostRatePerMtok <= 0 {
		cfg.CostRatePerMtok = DefaultCostRatePerMtok
	}
	return cfg, nil
}
