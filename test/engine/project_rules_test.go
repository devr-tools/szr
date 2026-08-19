package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/rules"
	"github.com/devr-tools/szr/test/testutil"
)

func TestProjectRulesProfileMerge(t *testing.T) {
	e, _, _ := newProjectRulesEngine(t)
	profiles := e.Profiles()
	if len(profiles) != 5 {
		t.Fatalf("unexpected merged profile count: %d", len(profiles))
	}
	gitStatusCount := 0
	for _, profile := range profiles {
		if profile.Name == "git-status" {
			gitStatusCount++
		}
	}
	if gitStatusCount != 1 {
		t.Fatalf("expected custom git-status to dedupe built-in, got %d", gitStatusCount)
	}
}

func TestProjectRulesExplainLocalRewrite(t *testing.T) {
	e, _, _ := newProjectRulesEngine(t)
	explained := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "target"},
		Display: []string{"argvdump", "target"},
	})
	if explained.Name != "argvdump-local" || len(explained.Explain) == 0 {
		t.Fatalf("unexpected explain result: %#v", explained)
	}
}

func TestProjectRulesDisplayWrapperMatch(t *testing.T) {
	e, _, worktree := newProjectRulesEngine(t)
	explained := e.Explain(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"summary", "npm", "test"},
		Cwd:     worktree,
	})
	if explained.Name != "display-wrapper" {
		t.Fatalf("expected display-prefix match, got %#v", explained)
	}
}

func TestProjectRulesExplainDecisions(t *testing.T) {
	e, _, _ := newProjectRulesEngine(t)
	decisions := e.ExplainDecisions(engine.Invocation{
		Command: []string{"git", "status"},
		Display: []string{"git", "status"},
	})
	if len(decisions) != 2 || !decisions[0].Selected || decisions[0].Source != engine.SourceProject || decisions[1].Source != engine.SourceBuiltin {
		t.Fatalf("unexpected explain decisions: %#v", decisions)
	}
}

func TestProjectRulesGitStatusOverride(t *testing.T) {
	e, _, _ := newProjectRulesEngine(t)
	explained := e.Explain(engine.Invocation{
		Command: []string{"git", "status"},
		Display: []string{"git", "status"},
	})
	if explained.Name != "git-status" || explained.Render == nil {
		t.Fatalf("expected custom git-status override, got %#v", explained)
	}
	if got := explained.Render(engine.Invocation{}, engine.Execution{Stdout: "one\ntwo\nthree\n"}); got != "one\n... +2 more lines" {
		t.Fatalf("unexpected compact render output: %q", got)
	}
}

func TestProjectRulesRewriteAndSkipGuard(t *testing.T) {
	e, root, _ := newProjectRulesEngine(t)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "target"},
		Display: []string{"argvdump", "target"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute argvdump with rewrite: %v", err)
	}
	if got := strings.Split(result.Display, "\n"); len(got) != 2 || got[0] != "target" || got[1] != "--json" {
		t.Fatalf("unexpected rewritten output: %q", result.Display)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "target", "--json"},
		Display: []string{"argvdump", "target", "--json"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute argvdump with skip flag: %v", err)
	}
	if strings.Count(result.Display, "--json") != 1 {
		t.Fatalf("expected rewrite skip guard, got %q", result.Display)
	}
}

func TestProjectRulesCannotRewriteShellWrappedCommand(t *testing.T) {
	e, root, _ := newProjectRulesEngine(t)
	args := []string{"sh", "-c", "argvdump target"}
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: args,
		Display: args,
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute shell-wrapped argvdump: %v", err)
	}
	if result.ProfileName != "argvdump-local" {
		t.Fatalf("expected project profile to render the inner command, got %q", result.ProfileName)
	}
	if strings.Contains(result.RawCombined, "--json") {
		t.Fatalf("project rewrite was spliced into shell wrapper: %q", result.RawCombined)
	}
	if !strings.Contains(result.RawCombined, "target") {
		t.Fatalf("expected original shell command to run, got %q", result.RawCombined)
	}
}

func TestProjectRulesFailureRender(t *testing.T) {
	e, root, _ := newProjectRulesEngine(t)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"noisy"},
		Display: []string{"noisy"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute noisy render: %v", err)
	}
	if !strings.Contains(result.Display, "FAIL first") {
		t.Fatalf("unexpected failure render output: %q", result.Display)
	}
	if strings.Contains(result.Display, "plain second") || strings.Contains(result.Display, "plain third") {
		t.Fatalf("expected failure-focused render, got %q", result.Display)
	}
}

func newProjectRulesEngine(t *testing.T) (*engine.Engine, string, string) {
	t.Helper()
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "argvdump", "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	testutil.WriteExecutable(t, binDir, "noisy", "#!/bin/sh\necho \"FAIL first\"\necho \"plain second\"\necho \"plain third\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	worktree := filepath.Join(root, "nested")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	cfg.ProjectRules = rules.File{
		Profiles: []rules.Profile{
			{
				Name:        "argvdump-local",
				Description: "Project-local arg rewriter",
				Explain:     []string{"Appends --json locally."},
				Match:       rules.Match{CommandPrefix: []string{"argvdump"}},
				Rewrite: rules.Rewrite{
					Mode:         "append",
					Args:         []string{"--json"},
					SkipIfHasAny: []string{"--json"},
				},
				Render: rules.Render{Mode: "passthrough"},
			},
			{
				Name: "display-wrapper",
				Match: rules.Match{
					DisplayPrefix: []string{"summary", "npm", "test"},
					CwdContains:   []string{"nested"},
				},
			},
			{
				Name:   "noisy-local",
				Match:  rules.Match{CommandPrefix: []string{"noisy"}},
				Render: rules.Render{Mode: "failure", MaxLines: 2},
			},
			{
				Name:   "git-status",
				Match:  rules.Match{CommandPrefix: []string{"git", "status"}},
				Render: rules.Render{Mode: "compact", MaxLines: 1},
			},
		},
	}

	builtins := []engine.Profile{
		{
			Name: "git-status",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) >= 2 && inv.Command[0] == "git" && inv.Command[1] == "status"
			},
			Render: func(engine.Invocation, engine.Execution) string { return "builtin" },
		},
		{
			Name: "fallback",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) > 0 && inv.Command[0] == "fallback"
			},
			Render: func(engine.Invocation, engine.Execution) string { return "fallback" },
		},
	}

	return engine.New(cfg, paths, history.New(paths.HistoryFile), builtins), root, worktree
}

func TestProjectRuleEdgeCases(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "argvdump", "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	cfg.ProjectRules = rules.File{
		Profiles: []rules.Profile{
			{
				Name:        "replace-local",
				Description: "replace rule",
				Match: rules.Match{
					CommandPrefix: []string{"argvdump"},
					AllArgs:       []string{"--target"},
					AnyArgs:       []string{"--replace"},
					ExcludeArgs:   []string{"--skip"},
				},
				Rewrite: rules.Rewrite{
					Mode: "replace",
					Args: []string{"argvdump", "replaced", "--json"},
				},
				Render: rules.Render{
					Mode: "compact",
				},
			},
			{
				Name: "default-render",
				Match: rules.Match{
					CommandPrefix: []string{"argvdump"},
					AnyArgs:       []string{"--default-render"},
				},
			},
		},
	}

	e := engine.New(cfg, paths, history.New(paths.HistoryFile), nil)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "--target", "--replace"},
		Display: []string{"argvdump", "--target", "--replace"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute replace rule: %v", err)
	}
	if !strings.Contains(result.Display, "replaced") || !strings.Contains(result.Display, "--json") {
		t.Fatalf("unexpected replaced display: %q", result.Display)
	}

	fallback := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "--target", "--replace", "--skip"},
		Display: []string{"argvdump", "--target", "--replace", "--skip"},
	})
	if fallback.Name != "passthrough" {
		t.Fatalf("expected excluded args to miss custom rule, got %#v", fallback)
	}

	alsoFallback := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "--replace"},
		Display: []string{"argvdump", "--replace"},
	})
	if alsoFallback.Name != "passthrough" {
		t.Fatalf("expected missing all_args to miss custom rule, got %#v", alsoFallback)
	}

	defaultRender, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "--default-render", "line1", "line2"},
		Display: []string{"argvdump", "--default-render", "line1", "line2"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute default render: %v", err)
	}
	if !strings.Contains(defaultRender.Display, "line1") || !strings.Contains(defaultRender.Display, "line2") {
		t.Fatalf("unexpected default render output: %q", defaultRender.Display)
	}
}
