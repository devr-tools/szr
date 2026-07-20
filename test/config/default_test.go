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
	if !cfg.Advanced.SessionDedup || cfg.Advanced.SessionDedupWindowMinutes != config.DefaultSessionDedupWindowMinutes {
		t.Fatalf("unexpected session dedup defaults: %#v", cfg.Advanced)
	}
	if !cfg.Advanced.DeltaRender {
		t.Fatalf("unexpected delta render default: %#v", cfg.Advanced)
	}
	if cfg.Diagnostics.Enabled || cfg.Diagnostics.Endpoint != "" || cfg.Diagnostics.MaxOutboxMB != config.DefaultDiagnosticsMaxOutboxMB {
		t.Fatalf("unexpected diagnostics defaults: %#v", cfg.Diagnostics)
	}
}

func TestNormalizeSessionDedupWindow(t *testing.T) {
	cfg := config.Default()
	cfg.Advanced.SessionDedupWindowMinutes = 0
	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Advanced.SessionDedupWindowMinutes != config.DefaultSessionDedupWindowMinutes {
		t.Fatalf("expected window normalization, got %#v", normalized.Advanced)
	}

	cfg.Advanced.SessionDedupWindowMinutes = 5
	normalized, err = config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.Advanced.SessionDedupWindowMinutes != 5 {
		t.Fatalf("expected custom window preserved, got %#v", normalized.Advanced)
	}
}
