package config_test

import (
	"testing"

	"szr/internal/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if !cfg.TeeOnFailure || cfg.MaxPreviewLines != 12 || cfg.MaxMatchGroups != 8 || cfg.ReasoningBudgetMode != config.ReasoningBudgetStandard {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}
