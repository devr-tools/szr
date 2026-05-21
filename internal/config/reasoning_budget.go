package config

import (
	"fmt"
	"strings"
)

const (
	ReasoningBudgetStandard = "standard"
	ReasoningBudgetAgent    = "agent"
)

func NormalizeReasoningBudgetMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", ReasoningBudgetStandard:
		return ReasoningBudgetStandard, nil
	case ReasoningBudgetAgent, "loop", "agent-loop":
		return ReasoningBudgetAgent, nil
	default:
		return "", fmt.Errorf("invalid reasoning budget mode %q (want %q or %q)", value, ReasoningBudgetStandard, ReasoningBudgetAgent)
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
	return cfg, nil
}
