package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/history"
	"szr/internal/teeindex"
	"szr/test/testutil"
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

func TestReplayAndCompareCommands(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths))

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

	code, stdout, stderr = testutil.RunApp(t, app, "replay", "100_git_diff")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "profile: git-diff") {
		t.Fatalf("unexpected replay tee stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\nif [ \"$1\" = \"diff\" ]; then\n  echo \"diff --git a/a.go b/a.go\"\n  echo \" a.go | 2 +-\"\n  echo \" 1 file changed, 1 insertion(+), 1 deletion(-)\"\nfi\n")
	code, stdout, stderr = testutil.RunApp(t, app, "compare", "git", "diff")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected compare stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"command: git diff", "effective command:", "profile: git-diff", "reduced preview:", "files=1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected compare output %q in %q", want, stdout)
		}
	}
}

func TestRulesScaffoldAndDoctorHistory(t *testing.T) {
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

	rulesPath := filepath.Join(root, ".szr.yaml")
	testutil.MustWriteFile(t, rulesPath, `version: 1
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

	restore := testutil.Chdir(t, root)
	defer restore()

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
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

	code, stdout, stderr = testutil.RunApp(t, app, "scaffold", "profile", "demo-profile", "--print")
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

	code, stdout, stderr = testutil.RunApp(t, app, "doctor", "--history")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor history stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"history diagnostics:", "commands: 3", "recommendations:", "hotspots:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected doctor history output %q in %q", want, stdout)
		}
	}
}
