package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/internal/profiles"
	"szr/internal/rules"
	"szr/test/testutil"
)

func TestExplainShowsProjectAndBuiltinDecisions(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	paths.ProjectRuleFile = filepath.Join(root, ".szr.yaml")
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	cfg.ProjectRules = rules.File{
		Profiles: []rules.Profile{
			{
				Name:        "local-npm-test",
				Description: "Project-local test wrapper",
				Explain:     []string{"Adds the repository-local test reporter."},
				Match: rules.Match{
					CommandPrefix: []string{"npm", "test"},
					CwdContains:   []string{filepath.Base(root)},
				},
			},
		},
	}

	store := history.New(paths.HistoryFile)
	app := cli.NewWithDependencies("test", cfg, paths, store, engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines)))

	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "explain", "npm", "test")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected explain stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"profile: local-npm-test",
		"source: project-local (" + paths.ProjectRuleFile + ")",
		"matched decisions:",
		"selected  project-local (" + paths.ProjectRuleFile + ")  local-npm-test",
		"also matches  built-in",
		"Adds the repository-local test reporter.",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected explain output to contain %q, got %q", want, stdout)
		}
	}
}
