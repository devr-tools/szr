package engine_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func newBuiltinEngine(t *testing.T) *engine.Engine {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	return engine.New(cfg, paths, history.New(paths.HistoryFile), profiles.Builtins(cfg.MaxPreviewLines))
}

func TestShellWrapperRoutesToInnerCommandProfiles(t *testing.T) {
	t.Parallel()
	e := newBuiltinEngine(t)

	cases := []struct {
		name    string
		args    []string
		profile string
	}{
		{"go test behind zsh login wrapper", []string{"/bin/zsh", "-lc", "source ~/.env && go test ./..."}, "go-test-json"},
		{"git status behind zsh wrapper", []string{"zsh", "-lc", "git status"}, "git-status"},
		{"git status with separate flags", []string{"bash", "-l", "-c", "git status"}, "git-status"},
		{"node eval direct", []string{"node", "-e", `throw new Error("boom")`}, "node-eval"},
		{"node eval behind wrapper", []string{"zsh", "-lc", "source /dev/null && node -e 'process.exit(1)'"}, "node-eval"},
		{"pipeline stays on fallback", []string{"zsh", "-lc", "git status | head -5"}, "passthrough"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := e.Explain(engine.Invocation{Command: tc.args, Display: tc.args})
			if profile.Name != tc.profile {
				t.Fatalf("expected %v to route to %q, got %q", tc.args, tc.profile, profile.Name)
			}
		})
	}
}

func newPrepareTranslationEngine(t *testing.T) *engine.Engine {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	return engine.New(config.Default(), paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name: "echo-append",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Command) > 0 && inv.Command[0] == "echo"
		},
		Prepare: func(inv engine.Invocation) []string {
			return append(append([]string(nil), inv.Command...), "appended")
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return strings.TrimSpace(exec.Stdout)
		},
	}})
}

// A literal-safe wrapper keeps the wrapper argv (setup segment included) and
// translates the profile's Prepare rewrite into the rebuilt command string.
func TestShellWrapperTranslatesPrepareIntoWrapper(t *testing.T) {
	t.Parallel()
	e := newPrepareTranslationEngine(t)
	args := []string{"/bin/sh", "-c", "SETUP_ONLY=1; echo hello"}
	result, err := e.Execute(context.Background(), engine.Invocation{Command: args, Display: args, Cwd: t.TempDir()}, false)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.ProfileName != "echo-append" {
		t.Fatalf("expected wrapper to match inner echo profile, got %q", result.ProfileName)
	}
	if !strings.Contains(result.RawCombined, "hello appended") {
		t.Fatalf("expected translated prepare rewrite in output, got %q", result.RawCombined)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}

// A wrapper whose final segment uses expansions cannot be rebuilt losslessly:
// the Prepare rewrite is suppressed and the original argv runs untouched.
func TestShellWrapperSuppressesPrepareForUnsafeInner(t *testing.T) {
	t.Parallel()
	e := newPrepareTranslationEngine(t)
	args := []string{"/bin/sh", "-c", "echo $HOME"}
	result, err := e.Execute(context.Background(), engine.Invocation{Command: args, Display: args, Cwd: t.TempDir()}, false)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.ProfileName != "echo-append" {
		t.Fatalf("expected wrapper to match inner echo profile, got %q", result.ProfileName)
	}
	if strings.Contains(result.RawCombined, "appended") {
		t.Fatalf("expected prepare suppression for unsafe inner, got %q", result.RawCombined)
	}
	if home := os.Getenv("HOME"); home != "" && !strings.Contains(result.RawCombined, home) {
		t.Fatalf("expected $HOME to expand in the original wrapper, got %q", result.RawCombined)
	}
}
