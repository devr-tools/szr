package config_test

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()
	if !cfg.TeeOnFailure || cfg.MaxPreviewLines != 12 || cfg.MaxMatchGroups != 8 || cfg.ReasoningBudgetMode != config.ReasoningBudgetStandard || cfg.UpdateCheck.Enabled || cfg.UpdateCheck.IntervalHours != 24 || cfg.UpdateCheck.AutoUpdate {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if !cfg.Advanced.AggressivePrepareRewrites || !cfg.Advanced.NoisePrefiltering || !cfg.Advanced.AdaptiveBudgets || !cfg.Advanced.EarlyCaptureStop || !cfg.Advanced.SemanticCompaction || !cfg.Advanced.CompressionContract || !cfg.Advanced.CompactArtifactRefs {
		t.Fatalf("unexpected advanced defaults: %#v", cfg.Advanced)
	}
}
