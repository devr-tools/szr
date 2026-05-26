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
	"github.com/devr-tools/szr/internal/teeindex"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecuteProfilesAndExplain(t *testing.T) {
	t.Parallel()
	e, _, _, _, _, _ := newExecuteTestEngine(t)
	if len(e.Profiles()) != 2 {
		t.Fatalf("unexpected profiles copy length")
	}
	explained := e.Explain(engine.Invocation{Display: []string{"other"}})
	if explained.Name != "passthrough" {
		t.Fatalf("unexpected fallback profile: %#v", explained)
	}
}

func TestExecuteMissingCommand(t *testing.T) {
	t.Parallel()
	e, _, _, _, _, _ := newExecuteTestEngine(t)
	if _, err := e.Execute(context.Background(), engine.Invocation{}, false); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestExecuteRenderedSuccess(t *testing.T) {
	t.Parallel()
	e, root, _, succeedPath, _, _ := newExecuteTestEngine(t)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{succeedPath},
		Display: []string{"succeed"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "rendered" || result.ExitCode != 0 || result.ProfileName != "custom" {
		t.Fatalf("unexpected success result: %#v err=%v", result, err)
	}
}

func TestExecuteBlankRenderFallsBackToRaw(t *testing.T) {
	t.Parallel()
	e, root, _, _, blankOutPath, _ := newExecuteTestEngine(t)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{blankOutPath},
		Display: []string{"blankout"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "raw-only" {
		t.Fatalf("expected raw fallback, got %#v err=%v", result, err)
	}
}

func TestExecuteFallbackProfileUsesDeclarativeCompactLines(t *testing.T) {
	t.Parallel()
	e, _, _, _, _, _ := newExecuteTestEngine(t)
	display := strings.Join([]string{
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
		"line-7",
		"line-8",
		"line-9",
		"line-10",
		"line-11",
		"line-12",
		"line-13",
	}, "\n")

	rendered := e.Explain(engine.Invocation{
		Command: []string{"other"},
		Display: []string{"other"},
	}).Render(engine.Invocation{}, engine.Execution{Stdout: display})

	if !strings.Contains(rendered, "line-1") || !strings.Contains(rendered, "... +1 more lines") {
		t.Fatalf("expected declarative compact fallback, got %q", rendered)
	}
	if strings.Contains(rendered, "line-13") {
		t.Fatalf("expected fallback compaction to truncate, got %q", rendered)
	}
}

func TestExecuteFallbackProfilePrefersInterestingErrorLinesOnFailure(t *testing.T) {
	t.Parallel()
	e, _, _, _, _, _ := newExecuteTestEngine(t)
	rendered := e.Explain(engine.Invocation{
		Command: []string{"other"},
		Display: []string{"other"},
	}).Render(engine.Invocation{}, engine.Execution{
		Stdout:   "progress line\nwarning: retrying\nplain line\nerror: failed to connect\n",
		ExitCode: 2,
	})

	if rendered != "warning: retrying\nerror: failed to connect" {
		t.Fatalf("expected declarative interesting-error fallback, got %q", rendered)
	}
}

func TestExecutePassthroughMode(t *testing.T) {
	t.Parallel()
	e, root, _, succeedPath, _, _ := newExecuteTestEngine(t)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{succeedPath},
		Display: []string{"succeed"},
		Cwd:     root,
	}, true)
	if err != nil || result.Display != "stdout" {
		t.Fatalf("expected passthrough result, got %#v err=%v", result, err)
	}
}

func TestExecuteTeeOnFailure(t *testing.T) {
	t.Parallel()
	e, root, paths, _, _, failPath := newExecuteTestEngine(t)
	failResult, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{failPath},
		Display: []string{"failcmd", strings.Repeat("x", 60)},
		Cwd:     root,
	}, false)
	if err != nil || failResult.ExitCode != 3 || failResult.TeePath == "" || !strings.Contains(failResult.Display, "[full output:") {
		t.Fatalf("unexpected failing result: %#v err=%v", failResult, err)
	}
	if _, statErr := os.Stat(failResult.TeePath); statErr != nil {
		t.Fatalf("expected tee file: %v", statErr)
	}
	indexEntries, err := teeindex.New(paths.TeeDir).List(10)
	if err != nil {
		t.Fatalf("list tee index: %v", err)
	}
	if len(indexEntries) != 1 || indexEntries[0].Path != failResult.TeePath || indexEntries[0].Command != "failcmd "+strings.Repeat("x", 60) {
		t.Fatalf("unexpected tee index entries: %#v", indexEntries)
	}
}

func TestExecuteTeeDisabled(t *testing.T) {
	t.Parallel()
	_, root, paths, _, _, failPath := newExecuteTestEngine(t)
	cfgNoTee := config.Default()
	cfgNoTee.TeeOnFailure = false
	eNoTee := engine.New(cfgNoTee, paths, history.New(filepath.Join(root, "data", "other.jsonl")), nil)
	noTeeResult, err := eNoTee.Execute(context.Background(), engine.Invocation{
		Command: []string{failPath},
		Display: []string{"failcmd"},
		Cwd:     root,
	}, false)
	if err != nil || noTeeResult.TeePath != "" {
		t.Fatalf("expected no tee result: %#v err=%v", noTeeResult, err)
	}
}

func TestExecuteExecError(t *testing.T) {
	t.Parallel()
	e, root, _, _, _, _ := newExecuteTestEngine(t)
	_, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"does-not-exist"},
		Display: []string{"does-not-exist"},
		Cwd:     root,
	}, false)
	if err == nil {
		t.Fatal("expected exec error")
	}
}

func newExecuteTestEngine(t *testing.T) (*engine.Engine, string, config.Paths, string, string, string) {
	t.Helper()
	binDir := t.TempDir()
	succeedPath := testutil.WriteExecutable(t, binDir, "succeed", "#!/bin/sh\necho stdout\n")
	failPath := testutil.WriteExecutable(t, binDir, "failcmd", "#!/bin/sh\necho stderr >&2\nexit 3\n")
	blankOutPath := testutil.WriteExecutable(t, binDir, "blankout", "#!/bin/sh\necho raw-only\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{
		{
			Name: "custom",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "succeed"
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return "rendered"
			},
		},
		{
			Name: "blank",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "blankout"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return ""
			},
		},
	})

	return e, root, paths, succeedPath, blankOutPath, failPath
}
