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

func TestExplainShowsAppliedPreferences(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	paths.ProjectRuleFile = filepath.Join(root, ".szr.json")
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	cfg.ProjectRules = rules.File{
		Preferences: []rules.Preference{
			{
				Name:        "internal-cli-json",
				Description: "Machine-readable output",
				Explain:     []string{"Prefers JSON for internal CLI output."},
				Match: rules.Match{
					CommandPrefix: []string{"internal-cli", "run"},
				},
				Rewrite: rules.Rewrite{
					Placement:    "before-terminator",
					Args:         []string{"--format", "json"},
					SkipIfHasAny: []string{"--format"},
				},
			},
		},
	}

	store := history.New(paths.HistoryFile)
	app := cli.NewWithDependencies("test", cfg, paths, store, engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines)))

	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "explain", "internal-cli", "run", "--", "target")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected explain stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"effective command: internal-cli run --format json -- target",
		"applied preferences:",
		"applied  project-preference (" + paths.ProjectRuleFile + ")  internal-cli-json",
		"profile: passthrough",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected explain output to contain %q, got %q", want, stdout)
		}
	}
}
