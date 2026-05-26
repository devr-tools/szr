package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/teeindex"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRecommendAndHotspotsCommands(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for i := 0; i < 4; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 21, 10+i, 0, 0, 0, time.UTC),
			Command:            "terraform plan",
			CommandFingerprint: history.Fingerprint("terraform plan"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         int64(15 + i),
			ExitCode:           1,
			RawTokens:          200,
			FilteredTokens:     12,
			SavedTokens:        188,
			SavingsPct:         94,
			FallbackUsed:       i < 3,
			TeePath:            "/tmp/terraform.log",
		}); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "recommend")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected recommend stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"recommendations:", "[budget] terraform plan", "[custom-profile] terraform plan", "[structured-rewrite] terraform plan"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected recommend output %q in %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "hotspots", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected hotspots json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode hotspots json: %v", err)
	}
	if len(payload) != 1 || payload[0]["command"] != "terraform plan" {
		t.Fatalf("unexpected hotspots payload: %#v", payload)
	}
}

func TestRecommendWrapperGuidanceForFindAndGrep(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	records := []history.Record{
		{
			Timestamp:          time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			Command:            "/usr/bin/find /repo -name \"users.py\"",
			CommandFingerprint: history.Fingerprint("/usr/bin/find /repo -name \"users.py\""),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         18,
			RawTokens:          80,
			FilteredTokens:     70,
			SavedTokens:        10,
			SavingsPct:         12.5,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
			Command:            "/usr/bin/find /repo -name \"users.py\"",
			CommandFingerprint: history.Fingerprint("/usr/bin/find /repo -name \"users.py\""),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         19,
			RawTokens:          82,
			FilteredTokens:     72,
			SavedTokens:        10,
			SavingsPct:         12.2,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
			Command:            "/usr/bin/grep -rn links_service /repo",
			CommandFingerprint: history.Fingerprint("/usr/bin/grep -rn links_service /repo"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         16,
			RawTokens:          90,
			FilteredTokens:     78,
			SavedTokens:        12,
			SavingsPct:         13.3,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC),
			Command:            "/usr/bin/grep -rn links_service /repo",
			CommandFingerprint: history.Fingerprint("/usr/bin/grep -rn links_service /repo"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         17,
			RawTokens:          92,
			FilteredTokens:     80,
			SavedTokens:        12,
			SavingsPct:         13.0,
		},
	}
	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append wrapper guidance history: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "recommend")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected wrapper recommend stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"[wrapper-guidance] /usr/bin/find /repo -name \"users.py\"", "szr find <path> --name ...", "[wrapper-guidance] /usr/bin/grep -rn links_service /repo", "szr grep <pattern> <path>"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected wrapper recommend output %q in %q", want, stdout)
		}
	}
}

func TestRecommendRoutingExpansionForSafeGitFamilies(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	records := []history.Record{
		{
			Timestamp:          time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --stat",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --stat"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         21,
			RawTokens:          90,
			FilteredTokens:     90,
			SavedTokens:        0,
			SavingsPct:         0,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --stat",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --stat"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         22,
			RawTokens:          92,
			FilteredTokens:     92,
			SavedTokens:        0,
			SavingsPct:         0,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --name-only",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --name-only"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         23,
			RawTokens:          80,
			FilteredTokens:     80,
			SavedTokens:        0,
			SavingsPct:         0,
		},
		{
			Timestamp:          time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC),
			Command:            "git diff HEAD~1..HEAD --stat | tail -30",
			CommandFingerprint: history.Fingerprint("git diff HEAD~1..HEAD --stat | tail -30"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         24,
			RawTokens:          95,
			FilteredTokens:     95,
			SavedTokens:        0,
			SavingsPct:         0,
		},
	}
	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append routing history: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "recommend")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected routing recommend stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"[routing-expansion] git diff",
		"representative rewrite: `szr proxy git diff HEAD~1..HEAD --stat | tail -30`",
		"[routing-coverage] git diff HEAD~1..HEAD --stat",
		"`szr git diff HEAD~1..HEAD --stat`",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected routing recommend output %q in %q", want, stdout)
		}
	}
}

func TestReplayCommandWithFile(t *testing.T) {
	root, paths, app := newWorkflowApp(t)
	diffPath := filepath.Join(root, "diff.log")
	testutil.MustWriteFile(t, diffPath, "diff --git a/a.go b/a.go\na/a.go | 2 +-\n1 file changed, 1 insertion(+), 1 deletion(-)\n")
	code, stdout, stderr := testutil.RunApp(t, app, "replay", diffPath, "--command", "git diff")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected replay stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"profile: git-diff", "rendered:", "files=1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected replay output %q in %q", want, stdout)
		}
	}
	_ = paths
}

func TestReplayCommandWithTee(t *testing.T) {
	root, paths, app := newWorkflowApp(t)
	store := teeindex.New(paths.TeeDir)
	teePath := filepath.Join(paths.TeeDir, "100_git_diff.log")
	testutil.MustWriteFile(t, teePath, "diff --git a/a.go b/a.go\na/a.go | 2 +-\n1 file changed, 1 insertion(+), 1 deletion(-)\n")
	if err := store.Append(teeindex.Entry{
		Timestamp: time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Path:      teePath,
		Command:   "git diff",
		Profile:   "git-diff",
		ExitCode:  1,
	}); err != nil {
		t.Fatalf("append tee entry: %v", err)
	}

	code, stdout, stderr := testutil.RunApp(t, app, "replay", "100_git_diff")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "profile: git-diff") {
		t.Fatalf("unexpected replay tee stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "replay", "100_git_diff", "--json", "--cwd", root, "--exit-code", "7", "--max-lines", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected replay json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var replayPayload map[string]any
	if err := json.Unmarshal([]byte(stdout), &replayPayload); err != nil {
		t.Fatalf("decode replay json: %v", err)
	}
	if replayPayload["exit_code"] != float64(7) || replayPayload["effective_command"] != "git diff" || replayPayload["profile"] != "git-diff" {
		t.Fatalf("unexpected replay payload: %#v", replayPayload)
	}
	if display, _ := replayPayload["display"].(string); !strings.Contains(display, "files=1") {
		t.Fatalf("expected summarized replay display, got %#v", replayPayload)
	}
}

func TestCompareCommand(t *testing.T) {
	_, _, app := newWorkflowApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\nif [ \"$1\" = \"diff\" ]; then\n  echo \"diff --git a/a.go b/a.go\"\n  echo \" a.go | 2 +-\"\n  echo \" 1 file changed, 1 insertion(+), 1 deletion(-)\"\nfi\n")
	code, stdout, stderr := testutil.RunApp(t, app, "compare", "git", "diff")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected compare stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"command: git diff", "effective command:", "profile: git-diff", "reduced preview:", "files=1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected compare output %q in %q", want, stdout)
		}
	}
}

func TestRulesCheckAndTest(t *testing.T) {
	root, _, app := newRulesWorkflowApp(t)
	writeWorkflowRulesFile(t, root)
	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "rules", "check")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected rules check stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"rules: ", ".szr.yaml", "profiles: 1", "preferences: 1", "status: valid"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected rules check output %q in %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "rules", "test", "go", "test", "./...")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected rules test stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"selected profile: project-go-test", "source: project-local (", ".szr.yaml", "preferences:", "applied add-json"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected rules test output %q in %q", want, stdout)
		}
	}
}

func TestScaffoldProfileWorkflow(t *testing.T) {
	root, _, app := newRulesWorkflowApp(t)
	writeWorkflowRulesFile(t, root)
	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "scaffold", "profile", "demo-profile", "--print")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "plan: scaffold profile demo-profile") {
		t.Fatalf("unexpected scaffold print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "scaffold", "profile", "demo-profile")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected scaffold apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, path := range []string{
		filepath.Join(root, ".szr", "scaffold", "demo-profile", "profile.yaml"),
		filepath.Join(root, ".szr", "scaffold", "demo-profile", "expected.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected scaffolded file %s: %v", path, err)
		}
	}
}

func TestDoctorHistoryWorkflow(t *testing.T) {
	root, _, app := newRulesWorkflowApp(t)
	writeWorkflowRulesFile(t, root)
	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "doctor", "--history")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor history stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"history diagnostics:", "commands: 3", "recommendations:", "hotspots:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected doctor history output %q in %q", want, stdout)
		}
	}
}

func writeWorkflowRulesFile(t *testing.T, root string) {
	t.Helper()
	testutil.MustWriteFile(t, filepath.Join(root, ".szr.yaml"), `version: 1
profiles:
  - name: project-go-test
    description: project override
    match:
      command_prefix:
        - go
        - test
    render:
      mode: failure
      max_lines: 6
preferences:
  - name: add-json
    match:
      command_prefix:
        - go
        - test
    rewrite:
      args:
        - -json
`)
}

func newWorkflowApp(t *testing.T) (string, config.Paths, *cli.App) {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths))
	return root, paths, app
}

func newRulesWorkflowApp(t *testing.T) (string, config.Paths, *cli.App) {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for i := 0; i < 3; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 21, 8+i, 0, 0, 0, time.UTC),
			Command:            "terraform plan",
			CommandFingerprint: history.Fingerprint("terraform plan"),
			Profile:            "passthrough",
			ProfileConfidence:  "low",
			DurationMS:         20,
			ExitCode:           1,
			RawTokens:          100,
			FilteredTokens:     60,
			SavedTokens:        40,
			SavingsPct:         40,
			FallbackUsed:       true,
			TeePath:            "/tmp/terraform.log",
		}); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}
	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	return root, paths, app
}
