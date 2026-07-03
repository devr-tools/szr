package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// failureSummary is deliberately larger than the ~96-token capture preview so
// the pre-fix never-worse guard would flip to the truncated head.
const truncatedCaptureFailureSummary = "FAILED TestAlphaValidation: expected sanitized widget payload but decoder returned partial frame with trailing garbage bytes after the second replay attempt\n" +
	"FAILED TestBetaReconnect: broker connection reset during replay window, retries exhausted after three attempts and the socket never recovered before the deadline\n" +
	"FAILED TestGammaCheckpoint: checkpoint manifest hash mismatch, expected 9f31c2 got 4485aa while restoring the third shard\n" +
	"3 failed, 37 passed, 2 skipped"

// passSummary exceeds the terse-render threshold while the (identical,
// fold-friendly) noise head compacts far smaller — the pre-fix swap bait.
const truncatedCapturePassSummary = "2000 passed, 0 failed across 34 packages\n" +
	"slowest: TestHugeIntegrationScenario 12.4s, TestColdCacheWarmup 8.9s, TestParallelReplayWindow 7.2s\n" +
	"coverage: 84.2% of statements, race detector enabled, no skipped tests detected in this run"

// Regression for the arena-v2 RC1 finding: with stream capture limited to the
// preview window, the never-worse-than-raw guard compared a correct failure
// summary against the truncated HEAD of the output and replaced it, dropping
// every failing test name. The guard must stand down when capture truncated.
func TestGuardDoesNotSwapSummaryForTruncatedCaptureOnFailure(t *testing.T) {
	t.Parallel()

	result := runTruncatedCaptureCommand(t, truncatedCaptureFailureSummary, false, "1")

	for _, name := range []string{"TestAlphaValidation", "TestBetaReconnect", "TestGammaCheckpoint"} {
		if !strings.Contains(result.Display, name) {
			t.Fatalf("expected failing test %s to survive truncated-capture guard, got %q", name, result.Display)
		}
	}
	if strings.Contains(result.Display, "widget pipeline stage completed") {
		t.Fatalf("expected head-of-output noise to stay out of the render, got %q", result.Display)
	}
}

// Regression for the arena-v2 RC1b finding: for a Prepare-rewritten command,
// the terse-render preference compacted the truncated capture head and
// replaced a correct pass summary with it. Truncated capture must disable it.
func TestTerseRenderPreferenceSkipsTruncatedCapture(t *testing.T) {
	t.Parallel()

	result := runTruncatedCaptureCommand(t, truncatedCapturePassSummary, true, "0")

	if !strings.Contains(result.Display, "2000 passed") {
		t.Fatalf("expected pass summary to survive terse-render preference, got %q", result.Display)
	}
}

func runTruncatedCaptureCommand(t *testing.T, summary string, rewrite bool, exitCode string) engine.Result {
	t.Helper()

	// Emit well over the 384-byte capture preview so captureTruncated=true.
	// The noise lines are IDENTICAL so CompactLines folds them to a single
	// "(xN)" line — making the truncated head look temptingly terse to the
	// pre-fix guards. Real runners put pass noise first, failures last.
	script := "#!/bin/sh\n" +
		"i=0\nwhile [ $i -lt 40 ]; do printf 'ok   widget pipeline stage completed without incident in this iteration\\n'; i=$((i+1)); done\n" +
		"printf 'FAILED TestAlphaValidation\\nFAILED TestBetaReconnect\\nFAILED TestGammaCheckpoint\\n'\n" +
		"exit " + exitCode + "\n"
	binDir := t.TempDir()
	runnerPath := testutil.WriteExecutable(t, binDir, "noisyrunner", script)

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	profile := engine.Profile{
		Name:       "noisy-runner",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "noisyrunner"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &countingSummaryReducer{summary: summary}
		},
	}
	if rewrite {
		profile.Prepare = func(inv engine.Invocation) []string {
			return append(append([]string{}, inv.Command...), "--machine-mode")
		}
	}

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{profile})
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{runnerPath},
		Display: []string{"noisyrunner"},
		Cwd:     root,
	}, false)
	if err != nil && exitCode == "0" {
		t.Fatalf("execute noisy runner: %v", err)
	}
	return result
}
