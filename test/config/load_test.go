package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
	"szr/test/testutil"
)

func TestLoadVariants(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)

	cfg, gotPaths, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
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
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
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
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, nil },
	)
	if err == nil {
		t.Fatal("expected resolve error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return errors.New("ensure fail") },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, nil },
	)
	if err == nil {
		t.Fatal("expected ensure error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, errors.New("read fail") },
	)
	if err == nil {
		t.Fatal("expected read error")
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return []byte("{bad"), nil },
	)
	if err == nil {
		t.Fatal("expected json error")
	}
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
