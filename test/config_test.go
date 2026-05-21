package test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
)

func TestConfigDefault(t *testing.T) {
	cfg := config.Default()
	if !cfg.TeeOnFailure || cfg.MaxPreviewLines != 12 || cfg.MaxMatchGroups != 8 {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestResolvePathsWithVariants(t *testing.T) {
	paths, err := config.ResolvePathsWith(
		func() (string, error) { return "/cfg", nil },
		func() (string, error) { return "/cache", nil },
		func() (string, error) { return "/home", nil },
	)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if paths.ConfigFile != "/cfg/szr/config.json" || paths.HistoryFile != "/cache/szr/history.jsonl" {
		t.Fatalf("unexpected paths: %#v", paths)
	}

	paths, err = config.ResolvePathsWith(
		func() (string, error) { return "", errors.New("no config") },
		func() (string, error) { return "", errors.New("no cache") },
		func() (string, error) { return "/home", nil },
	)
	if err != nil {
		t.Fatalf("resolve with fallback: %v", err)
	}
	if paths.ConfigDir != "/home/.config/szr" || paths.DataDir != "/home/.local/share/szr" {
		t.Fatalf("unexpected fallback paths: %#v", paths)
	}

	_, err = config.ResolvePathsWith(
		func() (string, error) { return "", errors.New("no config") },
		func() (string, error) { return "/cache", nil },
		func() (string, error) { return "", errors.New("no home") },
	)
	if err == nil || !strings.Contains(err.Error(), "user config directory") {
		t.Fatalf("expected config resolution error, got %v", err)
	}

	_, err = config.ResolvePathsWith(
		func() (string, error) { return "/cfg", nil },
		func() (string, error) { return "", errors.New("no cache") },
		func() (string, error) { return "", errors.New("no home") },
	)
	if err == nil || !strings.Contains(err.Error(), "user cache directory") {
		t.Fatalf("expected cache resolution error, got %v", err)
	}
}

func TestEnsurePathsWithAndEnsurePaths(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir: filepath.Join(root, "config"),
		DataDir:   filepath.Join(root, "data"),
		TeeDir:    filepath.Join(root, "tee"),
	}
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.TeeDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected dir %s: %v", dir, err)
		}
	}

	failErr := errors.New("mkdir fail")
	err := config.EnsurePathsWith(paths, func(string, os.FileMode) error { return failErr })
	if !errors.Is(err, failErr) {
		t.Fatalf("expected mkdir error, got %v", err)
	}
}

func TestLoadVariants(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}

	cfg, gotPaths, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	if gotPaths.ConfigFile != paths.ConfigFile || !cfg.TeeOnFailure {
		t.Fatalf("unexpected load default result: %#v %#v", cfg, gotPaths)
	}

	cfg, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func(string) ([]byte, error) {
			return []byte(`{"tee_on_failure":false,"max_preview_lines":7,"max_match_groups":4}`), nil
		},
	)
	if err != nil || cfg.TeeOnFailure || cfg.MaxPreviewLines != 7 || cfg.MaxMatchGroups != 4 {
		t.Fatalf("unexpected loaded config: %#v err=%v", cfg, err)
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return config.Paths{}, errors.New("resolve fail") },
		func(config.Paths) error { return nil },
		func(string) ([]byte, error) { return nil, nil },
	)
	if err == nil {
		t.Fatal("expected resolve error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return errors.New("ensure fail") },
		func(string) ([]byte, error) { return nil, nil },
	)
	if err == nil {
		t.Fatal("expected ensure error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func(string) ([]byte, error) { return nil, errors.New("read fail") },
	)
	if err == nil {
		t.Fatal("expected read error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func(string) ([]byte, error) { return []byte("{bad"), nil },
	)
	if err == nil {
		t.Fatal("expected json error")
	}
}

func TestResolvePathsAndLoadWrappers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)

	paths, err := config.ResolvePaths()
	if err != nil {
		t.Fatalf("resolve wrapper: %v", err)
	}
	if !strings.Contains(paths.ConfigDir, "szr") || !strings.Contains(paths.DataDir, "szr") {
		t.Fatalf("unexpected wrapper paths: %#v", paths)
	}

	cfg, gotPaths, err := config.Load()
	if err != nil {
		t.Fatalf("load wrapper: %v", err)
	}
	if gotPaths.ConfigFile == "" || !cfg.TeeOnFailure {
		t.Fatalf("unexpected wrapper load: %#v %#v", cfg, gotPaths)
	}
}
