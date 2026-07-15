package config_test

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestDefaultCostRate(t *testing.T) {
	cfg := config.Default()
	if cfg.CostRatePerMtok != config.DefaultCostRatePerMtok {
		t.Fatalf("expected default cost rate %v, got %v", config.DefaultCostRatePerMtok, cfg.CostRatePerMtok)
	}
}

func TestNormalizeCostRateZeroMeansDefault(t *testing.T) {
	cfg := config.Default()
	cfg.CostRatePerMtok = 0

	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.CostRatePerMtok != config.DefaultCostRatePerMtok {
		t.Fatalf("expected zero cost rate normalized to default, got %v", normalized.CostRatePerMtok)
	}
}

func TestNormalizeCostRateKeepsExplicitValue(t *testing.T) {
	cfg := config.Default()
	cfg.CostRatePerMtok = 12.5

	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.CostRatePerMtok != 12.5 {
		t.Fatalf("expected explicit cost rate preserved, got %v", normalized.CostRatePerMtok)
	}
}
