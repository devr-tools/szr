package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/internal/teeindex"
	"szr/test/testutil"
)

func TestExecuteAndHistory(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	succeedPath := testutil.WriteExecutable(t, binDir, "succeed", "#!/bin/sh\necho stdout\n")
	failPath := testutil.WriteExecutable(t, binDir, "failcmd", "#!/bin/sh\necho stderr >&2\nexit 3\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	profile := engine.Profile{
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
	}
	blankProfile := engine.Profile{
		Name: "blank",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "blankout"
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return ""
		},
	}
	blankOutPath := testutil.WriteExecutable(t, binDir, "blankout", "#!/bin/sh\necho raw-only\n")

	e := engine.New(cfg, paths, store, []engine.Profile{profile, blankProfile})
	if len(e.Profiles()) != 2 {
		t.Fatalf("unexpected profiles copy length")
	}
	explained := e.Explain(engine.Invocation{Display: []string{"other"}})
	if explained.Name != "passthrough" {
		t.Fatalf("unexpected fallback profile: %#v", explained)
	}

	if _, err := e.Execute(context.Background(), engine.Invocation{}, false); err == nil {
		t.Fatal("expected missing command error")
	}

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{succeedPath},
		Display: []string{"succeed"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "rendered" || result.ExitCode != 0 || result.ProfileName != "custom" {
		t.Fatalf("unexpected success result: %#v err=%v", result, err)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{blankOutPath},
		Display: []string{"blankout"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "raw-only" {
		t.Fatalf("expected raw fallback, got %#v err=%v", result, err)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{succeedPath},
		Display: []string{"succeed"},
		Cwd:     root,
	}, true)
	if err != nil || result.Display != "stdout" {
		t.Fatalf("expected passthrough result, got %#v err=%v", result, err)
	}

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

	cfgNoTee := cfg
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

	_, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"does-not-exist"},
		Display: []string{"does-not-exist"},
		Cwd:     root,
	}, false)
	if err == nil {
		t.Fatal("expected exec error")
	}
}
