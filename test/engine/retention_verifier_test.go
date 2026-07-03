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

// The retention verifier fixtures below are realistic failing-tool outputs
// where a deliberately weak profile drops every identifying token. Each raw
// body is padded past the verifier's arming threshold (the compression
// contract's minimum raw tokens) so the check runs, and each case lists the
// needles the repair section must restore.

const retentionWeakSummary = "run summary: issues detected (details trimmed for brevity)"

type retentionFixture struct {
	name    string
	raw     string
	needles []string
}

func retentionVerifierFixtures() []retentionFixture {
	return []retentionFixture{
		{
			name: "js-test-runner-failure",
			raw: retentionPadLines("PASS src/components/Toolbar.test.js", 14) + "\n" +
				"FAIL src/components/Button.test.js\n" +
				"  ● Button › renders the primary label\n" +
				"\n" +
				"    expect(received).toBe(expected)\n" +
				"\n" +
				"    Expected: \"Save\"\n" +
				"    Received: \"Sve\"\n" +
				"\n" +
				"      at Object.<anonymous> (src/components/Button.test.js:42:15)\n" +
				"\n" +
				"Tests:       1 failed, 11 passed, 12 total\n" +
				"Snapshots:   0 total\n" +
				"Time:        4.821 s\n",
			needles: []string{"Button.test.js", "Button.test.js:42", "1 failed"},
		},
		{
			name: "rust-build-error",
			raw: retentionPadLines("   Compiling replayd v0.4.1 (/work/replayd)", 12) + "\n" +
				"error[E0599]: no method named `commit_frame` found for struct `ReplayBuffer` in the current scope\n" +
				"   --> src/replay/buffer.rs:214:31\n" +
				"    |\n" +
				"214 |         self.inner.commit_frame(frame)\n" +
				"    |                    ^^^^^^^^^^^^ method not found in `ReplayBuffer`\n" +
				"    |\n" +
				"help: there is a method with a similar name: `commit_frames`\n" +
				"error: aborting due to 1 previous error\n",
			needles: []string{"E0599", "buffer.rs:214"},
		},
		{
			name: "python-test-failure",
			raw: retentionPadLines("tests/test_sessions.py ..........", 12) + "\n" +
				"=================================== FAILURES ===================================\n" +
				"____________________ test_login_rejects_bad_token ______________________________\n" +
				"    def test_login_rejects_bad_token():\n" +
				">       assert resp.status_code == 401\n" +
				"E       assert 200 == 401\n" +
				"tests/test_auth.py:57: AssertionError\n" +
				"FAILED tests/test_auth.py::test_login_rejects_bad_token - assert 200 == 401\n" +
				"=========================== 1 failed, 24 passed in 3.41s =======================\n",
			needles: []string{"test_auth.py:57", "test_auth.py::test_login_rejects_bad_token", "1 failed"},
		},
		{
			name: "dotnet-build-error",
			raw: retentionPadLines("  Determining projects to restore and resolving package graph entries...", 16) + "\n" +
				"/src/App/Services/SessionStore.cs(12,34): error CS1002: ; expected [/src/App/App.csproj]\n" +
				"Build FAILED.\n" +
				"\n" +
				"/src/App/Services/SessionStore.cs(12,34): error CS1002: ; expected [/src/App/App.csproj]\n" +
				"    0 Warning(s)\n" +
				"    1 Error(s)\n" +
				"\n" +
				"Time Elapsed 00:00:04.19\n",
			needles: []string{"CS1002", "1 error"},
		},
	}
}

func retentionPadLines(line string, count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// TestRetentionVerifierRepairsDroppedCriticalFacts is the payoff test: a weak
// profile renders a contentful summary that drops every identifying token,
// and the verifier must append the dropped critical lines without replacing
// the render.
func TestRetentionVerifierRepairsDroppedCriticalFacts(t *testing.T) {
	t.Parallel()
	for _, fixture := range retentionVerifierFixtures() {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			result := runRetentionFixture(t, fixture, config.Default(), 1)

			if !strings.Contains(result.Display, retentionWeakSummary) {
				t.Fatalf("expected weak render to survive (repair must not replace), got %q", result.Display)
			}
			if !strings.Contains(result.Display, "missing detail:") {
				t.Fatalf("expected repair section, got %q", result.Display)
			}
			for _, needle := range fixture.needles {
				if !strings.Contains(strings.ToLower(result.Display), strings.ToLower(needle)) {
					t.Fatalf("expected repaired display to restore %q, got %q", needle, result.Display)
				}
			}
			if result.VerifierRepairs == 0 || result.VerifierSkipped {
				t.Fatalf("expected repair telemetry, got %#v", result)
			}
		})
	}
}

// TestRetentionVerifierDisabledLeavesRenderAlone pins the config flag: with
// advanced.retention_verifier off, the weak render ships unrepaired.
func TestRetentionVerifierDisabledLeavesRenderAlone(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Advanced.RetentionVerifier = false
	result := runRetentionFixture(t, retentionVerifierFixtures()[1], cfg, 1)

	if strings.Contains(result.Display, "missing detail:") || result.VerifierRepairs != 0 {
		t.Fatalf("expected no repair with verifier disabled, got %#v", result)
	}
}

// TestRetentionVerifierLenientOnPassingCounts pins the success-exit leniency:
// a passing run whose render drops only summary counts must not be bloated
// with a repair section.
func TestRetentionVerifierLenientOnPassingCounts(t *testing.T) {
	t.Parallel()
	fixture := retentionFixture{
		name: "passing-run-counts-only",
		raw: retentionPadLines("module ok: replay pipeline stage completed without incident", 24) + "\n" +
			"Tests:       31 passed, 31 total\n" +
			"Time:        2.114 s\n",
	}
	result := runRetentionFixture(t, fixture, config.Default(), 0)

	if strings.Contains(result.Display, "missing detail:") || result.VerifierRepairs != 0 {
		t.Fatalf("expected counts-only leniency on success, got %#v", result)
	}
	if result.VerifierSkipped {
		t.Fatalf("expected verification to run (not skip) on full capture, got %#v", result)
	}
}

func runRetentionFixture(t *testing.T, fixture retentionFixture, cfg config.Config, exitCode int) engine.Result {
	t.Helper()

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	script := "#!/bin/sh\ncat <<'SZREOF'\n" + fixture.raw + "\nSZREOF\nexit " + itoaBenchmark(exitCode) + "\n"
	commandPath := testutil.WriteExecutable(t, root, "weak-"+fixture.name, script)

	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name: "weak-summarizer",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "weaktool"
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return retentionWeakSummary
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{commandPath},
		Display: []string{"weaktool"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute %s: %v", fixture.name, err)
	}
	if result.ExitCode != exitCode {
		t.Fatalf("expected exit %d, got %#v", exitCode, result)
	}
	return result
}

// TestRetentionVerifierSkipsTruncatedCaptureWithoutArtifact pins raw-source
// honesty: when streaming capture stopped at the preview limit and no tee
// artifact survived, verification must record a skip instead of judging the
// render against a partial preview.
func TestRetentionVerifierSkipsTruncatedCaptureWithoutArtifact(t *testing.T) {
	t.Parallel()

	result := runTruncatedCaptureCommand(t, truncatedCapturePassSummary, false, "0")

	if !result.VerifierSkipped {
		t.Fatalf("expected verifier skip telemetry for truncated capture without artifact, got %#v", result)
	}
	if strings.Contains(result.Display, "missing detail:") || result.VerifierRepairs != 0 {
		t.Fatalf("expected no repair against a truncated preview, got %#v", result)
	}
}

// TestRetentionVerifierRepairsFromTeeArtifact pins the artifact path: with a
// preview-truncated capture on a failing run, the verifier reads the tee
// artifact for the full raw stream and repairs the dropped failure names.
func TestRetentionVerifierRepairsFromTeeArtifact(t *testing.T) {
	t.Parallel()

	weakSummary := "runner finished with problems; consult the full log for specifics and rerun locally"
	result := runRetentionStreamingRunner(t, weakSummary)

	if result.VerifierSkipped {
		t.Fatalf("expected artifact-backed verification, got skip: %#v", result)
	}
	if !strings.Contains(result.Display, "missing detail:") || result.VerifierRepairs == 0 {
		t.Fatalf("expected artifact-backed repair, got %#v", result)
	}
	for _, name := range []string{"TestAlphaValidation", "TestBetaReconnect", "TestGammaCheckpoint"} {
		if !strings.Contains(result.Display, name) {
			t.Fatalf("expected repaired display to restore %s, got %q", name, result.Display)
		}
	}
	if !strings.Contains(result.Display, weakSummary) {
		t.Fatalf("expected weak render to survive alongside the repair, got %q", result.Display)
	}
}

// runRetentionStreamingRunner executes a noisy failing command through a
// stream profile whose capture stops at the preview limit; the persisted tee
// artifact is the only complete raw source the verifier can use.
func runRetentionStreamingRunner(t *testing.T, summary string) engine.Result {
	t.Helper()

	script := "#!/bin/sh\n" +
		"i=0\nwhile [ $i -lt 40 ]; do printf 'ok   replay pipeline stage completed without incident in this iteration\\n'; i=$((i+1)); done\n" +
		"printf 'FAILED TestAlphaValidation\\nFAILED TestBetaReconnect\\nFAILED TestGammaCheckpoint\\n'\n" +
		"exit 1\n"
	binDir := t.TempDir()
	runnerPath := testutil.WriteExecutable(t, binDir, "noisyfailrunner", script)

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	e := engine.New(config.Default(), paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "streamed-runner",
		Confidence: engine.ConfidenceMedium,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "noisyfailrunner"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &countingSummaryReducer{summary: summary}
		},
	}})
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{runnerPath},
		Display: []string{"noisyfailrunner"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute noisy fail runner: %v", err)
	}
	if result.ExitCode != 1 || result.TeePath == "" {
		t.Fatalf("expected failing run with persisted artifact, got %#v", result)
	}
	return result
}
