package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
)

func TestResolvePathsWithVariants(t *testing.T) {
	paths, err := config.ResolvePathsWith(
		func() (string, error) { return "/cfg", nil },
		func() (string, error) { return "/cache", nil },
		func() (string, error) { return "/home", nil },
	)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	if paths.ConfigFile != filepath.Join("/cfg", "szr", "config.json") || paths.HistoryFile != filepath.Join("/cache", "szr", "history.jsonl") {
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
	if paths.ConfigDir != filepath.Join("/home", ".config", "szr") || paths.DataDir != filepath.Join("/home", ".local", "share", "szr") {
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
