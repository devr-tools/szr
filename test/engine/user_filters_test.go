package engine_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func writeUserFilterSpec(t *testing.T, dir, file, name, prefix string) {
	t.Helper()
	testutil.MustWriteFile(t, filepath.Join(dir, file), `{
		"name": "`+name+`",
		"description": "Keeps warnings and errors.",
		"match": {"command_prefix": ["`+prefix+`"]},
		"keep_patterns": ["^(WARN|ERROR) "],
		"head": 4,
		"tail": 2
	}`)
}

func TestUserFilterPrecedenceGlobalBeforeProject(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	writeUserFilterSpec(t, globalDir, "aaa.json", "global-a", "toolg")
	writeUserFilterSpec(t, projectDir, "bbb.json", "project-b", "toolp")

	var warn bytes.Buffer
	loaded := engine.LoadUserFilterProfiles(engine.UserFilterSources{
		GlobalDir:      globalDir,
		ProjectDir:     projectDir,
		ProjectEnabled: true,
		Warn:           &warn,
	}, 12, map[string]struct{}{})
	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles, got %#v", loaded)
	}
	if loaded[0].Name != "global-a" || loaded[0].Source != engine.SourceUserFilter {
		t.Fatalf("expected global filter first, got %#v", loaded[0])
	}
	if loaded[1].Name != "project-b" || loaded[1].Source != engine.SourceProjectFilter {
		t.Fatalf("expected project filter second, got %#v", loaded[1])
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warnings: %s", warn.String())
	}
}

func TestUserFilterProjectTrustGate(t *testing.T) {
	projectDir := t.TempDir()
	writeUserFilterSpec(t, projectDir, "proj.json", "proj-filter", "toolp")

	var warn bytes.Buffer
	loaded := engine.LoadUserFilterProfiles(engine.UserFilterSources{
		ProjectDir:     projectDir,
		ProjectEnabled: false,
		Warn:           &warn,
	}, 12, map[string]struct{}{})
	if len(loaded) != 0 {
		t.Fatalf("expected no profiles when project filters disabled, got %#v", loaded)
	}
	notice := warn.String()
	if !strings.Contains(notice, "ignoring project filters") || !strings.Contains(notice, "advanced.project_filters") {
		t.Fatalf("expected disabled-notice with enable hint, got %q", notice)
	}
	if strings.Count(notice, "\n") != 1 {
		t.Fatalf("expected a one-line notice, got %q", notice)
	}

	warn.Reset()
	loaded = engine.LoadUserFilterProfiles(engine.UserFilterSources{
		ProjectDir:     t.TempDir(),
		ProjectEnabled: false,
		Warn:           &warn,
	}, 12, map[string]struct{}{})
	if len(loaded) != 0 || warn.Len() != 0 {
		t.Fatalf("expected silence for empty project dir, got %#v %q", loaded, warn.String())
	}
}

func TestUserFilterShadowProtection(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	writeUserFilterSpec(t, globalDir, "git-status.json", "git-status", "git")
	writeUserFilterSpec(t, globalDir, "dup.json", "dup", "toolg")
	writeUserFilterSpec(t, projectDir, "dup.json", "dup", "toolp")

	var warn bytes.Buffer
	loaded := engine.LoadUserFilterProfiles(engine.UserFilterSources{
		GlobalDir:      globalDir,
		ProjectDir:     projectDir,
		ProjectEnabled: true,
		Warn:           &warn,
	}, 12, map[string]struct{}{"git-status": {}})
	if len(loaded) != 1 || loaded[0].Name != "dup" || loaded[0].Source != engine.SourceUserFilter {
		t.Fatalf("expected only the global dup filter, got %#v", loaded)
	}
	notice := warn.String()
	if !strings.Contains(notice, "git-status") || !strings.Contains(notice, "collides") {
		t.Fatalf("expected builtin collision warning, got %q", notice)
	}
	if !strings.Contains(notice, "project filter dup") {
		t.Fatalf("expected project-vs-global collision warning, got %q", notice)
	}
}

func TestUserFilterEngineRoutingAndRender(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	filtersDir := filepath.Join(paths.ConfigDir, "filters")
	writeUserFilterSpec(t, filtersDir, "mytool-warnings.json", "mytool-warnings", "mytool")
	writeUserFilterSpec(t, filtersDir, "my-git.json", "my-git", "git")

	cfg := config.Default()
	eng := engine.New(cfg, paths, history.New(paths.HistoryFile), profiles.Builtins(cfg.MaxPreviewLines))

	inv := engine.Invocation{Command: []string{"mytool", "build"}, Display: []string{"mytool", "build"}}
	explained := eng.Explain(inv)
	if explained.Name != "mytool-warnings" || explained.Source != engine.SourceUserFilter {
		t.Fatalf("expected user filter to route mytool, got %#v", explained)
	}

	rendered := explained.Render(inv, engine.Execution{Stdout: "info a\nWARN slow\ninfo b\nERROR boom\n"})
	if rendered != "WARN slow\nERROR boom" {
		t.Fatalf("unexpected user filter render: %q", rendered)
	}

	gitInv := engine.Invocation{Command: []string{"git", "status"}, Display: []string{"git", "status"}}
	if selected := eng.Explain(gitInv); selected.Source != engine.SourceBuiltin {
		t.Fatalf("expected builtin to win over user filter for git, got %#v", selected)
	}

	decisions := eng.ExplainDecisions(inv)
	if len(decisions) != 1 || decisions[0].Name != "mytool-warnings" || !decisions[0].Selected {
		t.Fatalf("unexpected explain decisions: %#v", decisions)
	}
	if decisions[0].Source != engine.SourceUserFilter {
		t.Fatalf("expected user source annotation, got %#v", decisions[0])
	}
}

func TestUserFilterRegistersAfterBuiltins(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	writeUserFilterSpec(t, filepath.Join(paths.ConfigDir, "filters"), "mytool-warnings.json", "mytool-warnings", "mytool")

	cfg := config.Default()
	eng := engine.New(cfg, paths, history.New(paths.HistoryFile), profiles.Builtins(cfg.MaxPreviewLines))

	builtinCount := len(profiles.Builtins(cfg.MaxPreviewLines))
	list := eng.Profiles()
	if len(list) != builtinCount+1 {
		t.Fatalf("expected builtins plus one user filter, got %d profiles", len(list))
	}
	if list[len(list)-1].Name != "mytool-warnings" {
		t.Fatalf("expected user filter registered last, got %q", list[len(list)-1].Name)
	}
}
