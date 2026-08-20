package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSettingsProjectFiltersEntry(t *testing.T) {
	app := testutil.NewTestApp(t)

	var code int
	var stdout string
	testutil.WithStdin(t, "22\n1\nq\n", func() {
		code, stdout, _ = testutil.RunApp(t, app, "settings")
	})
	if code != 0 {
		t.Fatalf("settings exited with %d\n%s", code, stdout)
	}
	for _, want := range []string{
		"22. project filters",
		"saved: project filters enabled",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in settings output:\n%s", want, stdout)
		}
	}
}

func TestSettingsProjectFiltersPersisted(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	app := cli.NewWithDependencies("test", cfg, paths, store, eng)

	testutil.WithStdin(t, "22\n1\nq\n", func() {
		testutil.RunApp(t, app, "settings")
	})

	var saved config.Config
	if err := json.Unmarshal(testutil.MustReadFile(t, paths.ConfigFile), &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if !saved.Advanced.ProjectFilters {
		t.Fatalf("expected advanced.project_filters enabled, got %#v", saved.Advanced)
	}
}

func TestSettingsProjectRulesPersisted(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	app := cli.NewWithDependencies("test", cfg, paths, store, eng)

	testutil.WithStdin(t, "23\n1\nq\n", func() {
		testutil.RunApp(t, app, "settings")
	})

	var saved config.Config
	if err := json.Unmarshal(testutil.MustReadFile(t, paths.ConfigFile), &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if !saved.Advanced.ProjectRules {
		t.Fatalf("expected advanced.project_rules enabled, got %#v", saved.Advanced)
	}
}

func TestProfilesListsUserFilterWithSource(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	testutil.MustWriteFile(t, filepath.Join(paths.ConfigDir, "filters", "mytool-warnings.json"), `{
		"description": "Keeps warnings and errors from mytool runs.",
		"match": {"command_prefix": ["mytool"]},
		"keep_patterns": ["^(WARN|ERROR) "]
	}`)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	app := cli.NewWithDependencies("test", cfg, paths, store, eng)

	code, stdout, _ := testutil.RunApp(t, app, "profiles")
	if code != 0 {
		t.Fatalf("profiles exited with %d\n%s", code, stdout)
	}
	if !strings.Contains(stdout, "mytool-warnings") {
		t.Fatalf("expected user filter in profiles listing:\n%s", stdout)
	}
	if !strings.Contains(stdout, "source: "+engine.SourceUserFilter) {
		t.Fatalf("expected user source annotation in profiles listing:\n%s", stdout)
	}
}
