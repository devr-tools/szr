package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/test/testutil"
)

func TestLoadVariants(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)

	assertLoadVariantSuccesses(t, paths, root)
	assertLoadVariantErrors(t, paths, root)
}

func assertLoadVariantSuccesses(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	assertLoadDefaultConfig(t, paths, root)
	assertLoadCustomConfig(t, paths, root)
	assertLoadAggressiveConfig(t, paths, root)
	assertLoadNormalizedUpdateCheck(t, paths, root)
}

func assertLoadDefaultConfig(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	cfg, gotPaths, err := loadConfigFixture(t, paths, root, nil, nil, nil)
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	if gotPaths.ConfigFile != paths.ConfigFile || !cfg.TeeOnFailure {
		t.Fatalf("unexpected load default result: %#v %#v", cfg, gotPaths)
	}
}

func assertLoadCustomConfig(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	cfg, _, err := loadConfigFixture(t, paths, root, nil, nil, []byte(`{"tee_on_failure":false,"max_preview_lines":7,"max_match_groups":4,"reasoning_budget":"agent","advanced":{"adaptive_budgets":true,"early_capture_stop":false},"update_check":{"enabled":true,"interval_hours":12,"auto_update":true}}`))
	if err != nil {
		t.Fatalf("unexpected loaded config: %#v err=%v", cfg, err)
	}
	assertCustomConfigFields(t, cfg)
}

func assertLoadAggressiveConfig(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	cfg, _, err := loadConfigFixture(t, paths, root, nil, nil, []byte(`{"reasoning_budget":"aggressive"}`))
	if err != nil || cfg.ReasoningBudgetMode != config.ReasoningBudgetAggressive {
		t.Fatalf("unexpected aggressive loaded config: %#v err=%v", cfg, err)
	}
}

func assertLoadNormalizedUpdateCheck(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	cfg, _, err := loadConfigFixture(t, paths, root, nil, nil, []byte(`{"update_check":{"enabled":true,"interval_hours":0}}`))
	if err != nil || !cfg.UpdateCheck.Enabled || cfg.UpdateCheck.IntervalHours != 24 {
		t.Fatalf("expected normalized update check config, got %#v err=%v", cfg.UpdateCheck, err)
	}
}

func assertCustomConfigFields(t *testing.T, cfg config.Config) {
	t.Helper()

	if cfg.TeeOnFailure {
		t.Fatal("expected tee_on_failure to be false")
	}
	if cfg.MaxPreviewLines != 7 {
		t.Fatalf("expected max preview lines 7, got %d", cfg.MaxPreviewLines)
	}
	if cfg.MaxMatchGroups != 4 {
		t.Fatalf("expected max match groups 4, got %d", cfg.MaxMatchGroups)
	}
	if cfg.ReasoningBudgetMode != config.ReasoningBudgetAgent {
		t.Fatalf("expected reasoning budget %q, got %q", config.ReasoningBudgetAgent, cfg.ReasoningBudgetMode)
	}
	assertUpdateCheckConfig(t, cfg)
	assertAdvancedConfigDefaults(t, cfg.Advanced)
}

func assertUpdateCheckConfig(t *testing.T, cfg config.Config) {
	t.Helper()

	if !cfg.UpdateCheck.Enabled {
		t.Fatal("expected update check to be enabled")
	}
	if cfg.UpdateCheck.IntervalHours != 12 {
		t.Fatalf("expected update interval 12, got %d", cfg.UpdateCheck.IntervalHours)
	}
	if !cfg.UpdateCheck.AutoUpdate {
		t.Fatal("expected auto update to be enabled")
	}
}

func assertAdvancedConfigDefaults(t *testing.T, advanced config.Advanced) {
	t.Helper()

	if !advanced.AggressivePrepareRewrites {
		t.Fatal("expected aggressive prepare rewrites")
	}
	if !advanced.NoisePrefiltering {
		t.Fatal("expected noise prefiltering")
	}
	if !advanced.AdaptiveBudgets {
		t.Fatal("expected adaptive budgets")
	}
	if advanced.EarlyCaptureStop {
		t.Fatal("expected early capture stop to be false")
	}
	if !advanced.SemanticCompaction {
		t.Fatal("expected semantic compaction")
	}
	if !advanced.CompressionContract {
		t.Fatal("expected compression contract")
	}
	if !advanced.CompactArtifactRefs {
		t.Fatal("expected compact artifact refs")
	}
}

func assertLoadVariantErrors(t *testing.T, paths config.Paths, root string) {
	t.Helper()
	for _, tc := range []struct {
		name        string
		resolveErr  error
		ensureErr   error
		readErr     error
		data        []byte
		errContains string
	}{
		{name: "resolve error", resolveErr: errors.New("resolve fail")},
		{name: "ensure error", ensureErr: errors.New("ensure fail")},
		{name: "read error", readErr: errors.New("read fail")},
		{name: "json error", data: []byte("{bad")},
		{name: "reasoning budget conflict", data: []byte(`{"reasoning_budget":"agent","reasoning_budget_mode":"standard"}`), errContains: "reasoning_budget and reasoning_budget_mode disagree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := loadConfigFixture(t, paths, root, tc.resolveErr, tc.ensureErr, tc.data, tc.readErr)
			if err == nil {
				t.Fatal("expected error")
			}
			if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

func loadConfigFixture(t *testing.T, paths config.Paths, root string, resolveErr, ensureErr error, args ...any) (config.Config, config.Paths, error) {
	t.Helper()
	var data []byte
	var readErr error
	for _, arg := range args {
		switch v := arg.(type) {
		case []byte:
			data = v
		case error:
			readErr = v
		}
	}
	return config.LoadWith(
		func() (config.Paths, error) {
			if resolveErr != nil {
				return config.Paths{}, resolveErr
			}
			return paths, nil
		},
		func(config.Paths) error { return ensureErr },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) {
			if readErr != nil {
				return nil, readErr
			}
			if data != nil {
				return data, nil
			}
			return nil, os.ErrNotExist
		},
	)
}

func TestLoadEdgeErrors(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)

	_, _, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return "", errors.New("cwd fail") },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if err == nil || !strings.Contains(err.Error(), "cwd fail") {
		t.Fatalf("expected getwd error, got %v", err)
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, errors.New("stat fail") },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if err == nil || !strings.Contains(err.Error(), "stat fail") {
		t.Fatalf("expected discover stat error, got %v", err)
	}

	projectRoot := filepath.Join(root, "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	testutil.MustWriteFile(t, filepath.Join(projectRoot, ".szr.json"), `{"profiles":[{"name":"ok","match":{"command_prefix":["npm"]}}]}`)

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return projectRoot, nil },
		os.Stat,
		func(name string) ([]byte, error) {
			if strings.HasSuffix(name, ".szr.json") {
				return nil, errors.New("project read fail")
			}
			return nil, os.ErrNotExist
		},
	)
	if err == nil || !strings.Contains(err.Error(), "project read fail") {
		t.Fatalf("expected project rule read error, got %v", err)
	}
}
