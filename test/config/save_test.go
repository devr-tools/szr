package config_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSaveWritesNormalizedConfig(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	cfg := config.Default()
	cfg.ReasoningBudgetMode = "default"
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.IntervalHours = 0
	cfg.UpdateCheck.AutoUpdate = true
	cfg.MaxPreviewLines = 9
	cfg.Advanced.AdaptiveBudgets = true
	cfg.Advanced.EarlyCaptureStop = false

	if err := config.Save(paths, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	data := testutil.MustReadFile(t, paths.ConfigFile)
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("expected trailing newline, got %q", string(data))
	}

	var saved config.Config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if saved.ReasoningBudgetMode != config.ReasoningBudgetStandard {
		t.Fatalf("expected normalized reasoning mode, got %#v", saved)
	}
	if !saved.UpdateCheck.Enabled || !saved.UpdateCheck.AutoUpdate || saved.UpdateCheck.IntervalHours != 24 {
		t.Fatalf("expected normalized update config, got %#v", saved.UpdateCheck)
	}
	if saved.MaxPreviewLines != 9 {
		t.Fatalf("expected saved max preview lines, got %#v", saved)
	}
	if !saved.Advanced.AdaptiveBudgets || saved.Advanced.EarlyCaptureStop {
		t.Fatalf("expected saved advanced config, got %#v", saved.Advanced)
	}
}

func TestSavePropagatesEnsureError(t *testing.T) {
	err := config.SaveWith(
		config.Paths{},
		config.Default(),
		func(config.Paths) error { return os.ErrPermission },
		func(string, []byte, os.FileMode) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("expected ensure error, got %v", err)
	}
}
