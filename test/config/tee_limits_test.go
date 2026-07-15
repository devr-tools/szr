package config_test

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
)

func TestDefaultTeeRetention(t *testing.T) {
	cfg := config.Default()
	if cfg.TeeMaxFileMB != config.DefaultTeeMaxFileMB {
		t.Fatalf("expected default tee max file mb %d, got %d", config.DefaultTeeMaxFileMB, cfg.TeeMaxFileMB)
	}
	if cfg.TeeMaxDirFiles != config.DefaultTeeMaxDirFiles {
		t.Fatalf("expected default tee max dir files %d, got %d", config.DefaultTeeMaxDirFiles, cfg.TeeMaxDirFiles)
	}
	if cfg.TeeMaxDirMB != config.DefaultTeeMaxDirMB {
		t.Fatalf("expected default tee max dir mb %d, got %d", config.DefaultTeeMaxDirMB, cfg.TeeMaxDirMB)
	}
}

func TestNormalizeTeeRetentionZeroMeansDefault(t *testing.T) {
	cfg := config.Default()
	cfg.TeeMaxFileMB = 0
	cfg.TeeMaxDirFiles = -1
	cfg.TeeMaxDirMB = 0

	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.TeeMaxFileMB != config.DefaultTeeMaxFileMB ||
		normalized.TeeMaxDirFiles != config.DefaultTeeMaxDirFiles ||
		normalized.TeeMaxDirMB != config.DefaultTeeMaxDirMB {
		t.Fatalf("expected zero values normalized to defaults, got %#v", normalized)
	}
}

func TestNormalizeTeeRetentionKeepsExplicitValues(t *testing.T) {
	cfg := config.Default()
	cfg.TeeMaxFileMB = 8
	cfg.TeeMaxDirFiles = 50
	cfg.TeeMaxDirMB = 64

	normalized, err := config.Normalize(cfg)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if normalized.TeeMaxFileMB != 8 || normalized.TeeMaxDirFiles != 50 || normalized.TeeMaxDirMB != 64 {
		t.Fatalf("expected explicit values preserved, got %#v", normalized)
	}
}
