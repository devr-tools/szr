package engine_test

import (
	"context"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecuteBenignExitKeepsTinyBypassAndSkipsTee(t *testing.T) {
	t.Parallel()

	result := runBenignExitCommand(t, "#!/bin/sh\nprintf 'no matches here\\n'\nexit 1\n")
	if result.ExitCode != 1 {
		t.Fatalf("expected recorded exit code 1, got %#v", result)
	}
	if result.BypassReason == "" {
		t.Fatalf("expected benign exit to keep tiny-output bypass, got %#v", result)
	}
	if result.Display != "no matches here" {
		t.Fatalf("expected raw tiny output without failure escape, got %#v", result)
	}
	if result.TeePath != "" {
		t.Fatalf("expected no tee artifact for benign exit, got %#v", result)
	}
}

func TestExecuteNonBenignExitStillTreatedAsFailure(t *testing.T) {
	t.Parallel()

	result := runBenignExitCommand(t, "#!/bin/sh\nprintf 'rg: real failure while searching\\n' >&2\nexit 2\n")
	if result.ExitCode != 2 {
		t.Fatalf("expected recorded exit code 2, got %#v", result)
	}
	if result.BypassReason != "" {
		t.Fatalf("expected no fast-path bypass on non-benign failure, got %#v", result)
	}
	if result.TeePath == "" {
		t.Fatalf("expected tee artifact for non-benign failure, got %#v", result)
	}
}

func runBenignExitCommand(t *testing.T, script string) engine.Result {
	t.Helper()

	binDir := t.TempDir()
	commandPath := testutil.WriteExecutable(t, binDir, "rgish", script)

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:         "rgish",
		Confidence:   engine.ConfidenceMedium,
		Capabilities: engine.ProfileCapabilities{BenignExitCodes: []int{1}},
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "rgish"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &countingSummaryReducer{summary: "1 matches across 1 files"}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"rgish", "pattern"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute rgish: %v", err)
	}
	return result
}
