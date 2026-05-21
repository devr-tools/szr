package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
	"szr/test/testutil"
)

func TestLoadWithProjectRules(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "repo")
	worktree := filepath.Join(projectRoot, "pkg", "nested")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	paths := testutil.Paths(root)
	if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	testutil.MustWriteFile(t, paths.ConfigFile, `{"max_preview_lines":9}`)

	ruleFile := filepath.Join(projectRoot, ".szr.json")
	testutil.MustWriteFile(t, ruleFile, `{"version":1,"profiles":[{"name":"local-node","match":{"command_prefix":["npm","test"]},"rewrite":{"mode":"append","args":["--runInBand"]},"render":{"mode":"failure","max_lines":5}}]}`)

	cfg, gotPaths, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return worktree, nil },
		os.Stat,
		os.ReadFile,
	)
	if err != nil {
		t.Fatalf("load with project rules: %v", err)
	}
	if gotPaths.ProjectDir != projectRoot || gotPaths.ProjectRuleFile != ruleFile {
		t.Fatalf("unexpected project paths: %#v", gotPaths)
	}
	if cfg.MaxPreviewLines != 9 {
		t.Fatalf("expected config override, got %#v", cfg)
	}
	if len(cfg.ProjectRules.Profiles) != 1 || cfg.ProjectRules.Profiles[0].Name != "local-node" {
		t.Fatalf("unexpected project rules: %#v", cfg.ProjectRules)
	}
}

func TestLoadWithProjectRulesWithoutUserConfig(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	paths := testutil.Paths(root)
	ruleFile := filepath.Join(projectRoot, ".szr.json")
	testutil.MustWriteFile(t, ruleFile, `{"profiles":[{"name":"local","match":{"command_prefix":["pnpm","lint"]}}]}`)

	cfg, gotPaths, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return projectRoot, nil },
		os.Stat,
		os.ReadFile,
	)
	if err != nil {
		t.Fatalf("load with project-only rules: %v", err)
	}
	if !cfg.TeeOnFailure || len(cfg.ProjectRules.Profiles) != 1 {
		t.Fatalf("unexpected config with project-only rules: %#v", cfg)
	}
	if gotPaths.ProjectRuleFile != ruleFile {
		t.Fatalf("unexpected project rule path: %#v", gotPaths)
	}
}

func TestLoadWithProjectRuleErrors(t *testing.T) {
	root := t.TempDir()
	projectRoot := filepath.Join(root, "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	paths := testutil.Paths(root)
	yamlFile := filepath.Join(projectRoot, ".szr.yaml")
	testutil.MustWriteFile(t, yamlFile, "profiles: []\n")

	_, _, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return projectRoot, nil },
		os.Stat,
		os.ReadFile,
	)
	if err == nil || !strings.Contains(err.Error(), "yaml project rules are not supported") {
		t.Fatalf("expected yaml error, got %v", err)
	}

	if err := os.Remove(yamlFile); err != nil {
		t.Fatalf("remove yaml file: %v", err)
	}
	testutil.MustWriteFile(t, filepath.Join(projectRoot, ".szr.json"), `{"profiles":[{"name":"","match":{"command_prefix":["npm"]}}]}`)

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return projectRoot, nil },
		os.Stat,
		os.ReadFile,
	)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected validation error, got %v", err)
	}
}
