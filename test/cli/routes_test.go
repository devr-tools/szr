package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/updates"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRunRoutes(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\ncase \"$1\" in\nstatus)\n  echo \"## main...origin/main\"\n  echo \"M  README.md\"\n  ;;\nlog)\n  echo \"abc123 first\"\n  echo \"def456 second\"\n  ;;\ndiff)\n  echo \"diff --git a/a.go b/a.go\"\n  echo \" a.go | 2 +-\"\n  echo \" 1 file changed, 1 insertion(+), 1 deletion(-)\"\n  ;;\nesac\n")
	testutil.WriteExecutable(t, binDir, "go", "#!/bin/sh\ncase \"$1\" in\ntest)\n  echo '{\"Action\":\"pass\",\"Package\":\"pkg/pass\"}'\n  echo '{\"Action\":\"fail\",\"Package\":\"pkg/fail\"}'\n  echo '{\"Action\":\"fail\",\"Package\":\"pkg/fail\",\"Test\":\"TestSad\"}'\n  ;;\nbuild)\n  echo 'compile error' >&2\n  exit 1\n  ;;\nvet)\n  echo 'warning: suspicious' >&2\n  exit 1\n  ;;\nesac\n")
	testutil.WriteExecutable(t, binDir, "echoer", "#!/bin/sh\necho plain-output\n")
	testutil.WriteExecutable(t, binDir, "noisy", "#!/bin/sh\necho FAIL one\necho note >&2\n")
	testutil.WriteExecutable(t, binDir, "rg", "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"__error__\" ]; then\n    echo \"bad rg\" >&2\n    exit 2\n  fi\n  if [ \"$arg\" = \"nomatch\" ]; then\n    exit 1\n  fi\ndone\nif [ \"$1\" = \"__missing__\" ]; then\n  exit 1\nfi\necho \"file.go:12:match one\"\necho \"file.go:20:match two\"\n")

	root := t.TempDir()
	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.go")
	jsonFile := filepath.Join(root, "data.json")
	logFile := filepath.Join(root, "app.log")
	if err := os.WriteFile(fileA, []byte("one\n// c\n# d\n"), 0o644); err != nil {
		t.Fatalf("write fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("func x() { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write fileB: %v", err)
	}
	if err := os.WriteFile(jsonFile, []byte(`{"a":"x","b":[{"c":1}]}`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	if err := os.WriteFile(logFile, []byte("same\nsame\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dir", "sub", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
		stdin      string
	}{
		{"help empty", nil, 0, []string{"vtest", `szr or "sizer" is a token-aware CLI proxy built in Go`, "Setup:"}, nil, ""},
		{"help flag", []string{"--help"}, 0, []string{"Setup:", "Insight:", "Discover:", "szr commands", "--reasoning-budget <standard|agent>", "szr uninstall", "szr uninstall codex|claude-code|cursor|..."}, nil, ""},
		{"help ultra", []string{"-u", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose long", []string{"--verbose", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose exact", []string{"-vv", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose counted", []string{"-vvvv", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"commands", []string{"commands"}, 0, []string{"commands", "vtest", "Execution:", "Local Tools:", "Install:", "szr rg <pattern> [path]", "szr uninstall codex"}, nil, ""},
		{"commands rewrite", []string{"commands"}, 0, []string{"Integrations:", "szr rewrite --json --command '<cmd>'"}, nil, ""},
		{"version", []string{"--version"}, 0, []string{"szr test"}, nil, ""},
		{"profiles", []string{"profiles"}, 0, []string{"git-status", "generic-summary"}, nil, ""},
		{"doctor", []string{"doctor"}, 0, []string{"version: test", "reasoning budget mode: standard", "update checks: disabled", "go:", "git:", "rg:"}, nil, ""},
		{"self doctor", []string{"self", "doctor"}, 0, []string{"version: test", "install target:", "config dir:", "update checks: disabled"}, nil, ""},
		{"doctor missing tool", []string{"doctor"}, 0, []string{"go: missing"}, nil, ""},
		{"git status", []string{"git", "status"}, 0, []string{"main...origin/main", "M  README.md"}, nil, ""},
		{"git log", []string{"git", "log"}, 0, []string{"2 commits"}, nil, ""},
		{"git diff", []string{"git", "diff"}, 0, []string{"files=1 +1 -1", "a.go | 2 +-"}, nil, ""},
		{"go test", []string{"go", "test", "./..."}, 0, []string{"pkg/fail", "TestSad"}, nil, ""},
		{"go build", []string{"go", "build"}, 1, []string{"compile error"}, nil, ""},
		{"go vet", []string{"go", "vet"}, 1, []string{"warning: suspicious"}, nil, ""},
		{"run default route", []string{"echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"run explicit", []string{"run", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"proxy", []string{"proxy", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"test wrapper", []string{"test", "noisy"}, 0, []string{"FAIL one"}, nil, ""},
		{"summary wrapper", []string{"summary", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"rewrite command", []string{"rewrite", "--command", "git diff HEAD~1..HEAD --stat"}, 0, []string{"szr git diff HEAD~1..HEAD --stat"}, nil, ""},
		{"rewrite hint", []string{"rewrite", "--format", "hint", "--command", "/usr/bin/grep -rn needle ."}, 0, []string{"szr grep <pattern> <path>"}, nil, ""},
		{"rewrite pipeline", []string{"rewrite", "--command", "git diff HEAD~1..HEAD --stat | tail -30"}, 0, []string{"szr proxy git diff HEAD~1..HEAD --stat | tail -30"}, nil, ""},
		{"rewrite json", []string{"rewrite", "--format", "json", "--command", "git diff HEAD~1..HEAD --stat | tail -30"}, 0, []string{`"auto_rewrite":true`, `"wrap_mode":"proxy"`, `"producer_only":true`}, nil, ""},
		{"explain", []string{"explain", "git", "status"}, 0, []string{"profile: git-status"}, nil, ""},
		{"ls", []string{"ls", root}, 0, []string{filepath.Base(root), "dir", "sub"}, nil, ""},
		{"ls default root", []string{"ls"}, 0, []string{filepath.Base(".")}, nil, ""},
		{"find", []string{"find", root, "--name", "*.go"}, 0, []string{"1 matches", "b.go"}, nil, ""},
		{"find path filter", []string{"find", root, "--name", "*.txt", "--path", "*a.txt"}, 0, []string{"1 matches", "a.txt"}, nil, ""},
		{"find exclude", []string{"find", root, "--exclude", "dir/*"}, 0, []string{"5 matches"}, nil, ""},
		{"find max depth", []string{"find", root, "--type", "d", "--max-depth", "1"}, 0, []string{"1 matches", filepath.ToSlash(filepath.Join(root, "dir"))}, nil, ""},
		{"read single", []string{"read", fileA}, 0, []string{"one", "// c"}, nil, ""},
		{"read multi aggressive", []string{"read", "-l", "aggressive", "-n", "--max-lines", "1", fileA, fileB}, 0, []string{"== " + fileA + " ==", "== " + fileB + " ==", "func x() { ... }"}, nil, ""},
		{"grep", []string{"grep", "match", "."}, 0, []string{"file.go (2 matches)"}, nil, ""},
		{"grep default path", []string{"grep", "match"}, 0, []string{"file.go (2 matches)"}, nil, ""},
		{"rg external", []string{"rg", "match", "."}, 0, []string{"file.go (2 matches)"}, nil, ""},
		{"json", []string{"json", jsonFile}, 0, []string{"a: string", "c: number"}, nil, ""},
		{"log file", []string{"log", logFile}, 0, []string{"same (x2)"}, nil, ""},
		{"log stdin", []string{"log"}, 0, []string{"same (x2)"}, nil, "same\nsame\n"},
		{"verbose", []string{"-vvv", "run", "echoer"}, 0, []string{"plain-output"}, []string{"[szr] profile=passthrough", "[szr] raw:"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "doctor missing tool" {
				t.Setenv("PATH", t.TempDir())
			}

			var code int
			var stdout, stderr string
			run := func() {
				code, stdout, stderr = testutil.RunApp(t, app, tc.args...)
			}
			if tc.stdin != "" {
				testutil.WithStdin(t, tc.stdin, run)
			} else {
				run()
			}

			if code != tc.wantCode {
				t.Fatalf("unexpected code: got %d want %d stdout=%q stderr=%q", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
				}
			}
		})
	}
}

func TestRGExternalMissingShowsInstallHint(t *testing.T) {
	app := testutil.NewTestApp(t)
	t.Setenv("PATH", t.TempDir())

	code, stdout, stderr := testutil.RunApp(t, app, "rg", "needle", ".")
	if code != 1 || stdout != "" {
		t.Fatalf("unexpected rg missing stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"szr: `rg` is not installed or not on PATH",
		"szr: install ripgrep to use `szr rg ...`",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestDoctorMarksRipgrepOptional(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\nexit 0\n")
	testutil.WriteExecutable(t, binDir, "go", "#!/bin/sh\nexit 0\n")

	code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "rg: missing (optional; only needed for `szr rg`)") {
		t.Fatalf("expected optional rg status, got %q", stdout)
	}
}

func TestRewriteHookCursor(t *testing.T) {
	app := testutil.NewTestApp(t)
	var code int
	var stdout, stderr string
	testutil.WithStdin(t, `{"tool_name":"Bash","tool_input":{"command":"git diff HEAD~1..HEAD --stat | tail -30"}}`, func() {
		code, stdout, stderr = testutil.RunApp(t, app, "rewrite", "--hook", "cursor")
	})
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected rewrite hook cursor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{`"permission":"allow"`, `szr proxy git diff HEAD~1..HEAD --stat | tail -30`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected rewrite hook output %q in %q", want, stdout)
		}
	}
}

func TestRewriteReadsCommandFromStdin(t *testing.T) {
	app := testutil.NewTestApp(t)
	var code int
	var stdout, stderr string
	testutil.WithStdin(t, "git diff HEAD~1..HEAD --stat\n", func() {
		code, stdout, stderr = testutil.RunApp(t, app, "rewrite", "--stdin", "--binary", "custom-szr")
	})
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected rewrite stdin stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "custom-szr git diff HEAD~1..HEAD --stat") {
		t.Fatalf("expected stdin rewrite output, got %q", stdout)
	}
}

func TestRewriteHookFallbacks(t *testing.T) {
	app := testutil.NewTestApp(t)

	t.Run("cursor fallback", func(t *testing.T) {
		code, stdout, stderr := testutil.RunApp(t, app, "rewrite", "--hook", "cursor")
		if code != 0 || stderr != "" || strings.TrimSpace(stdout) != "{}" {
			t.Fatalf("unexpected cursor fallback stdout=%q stderr=%q code=%d", stdout, stderr, code)
		}
	})

	t.Run("gemini fallback", func(t *testing.T) {
		var code int
		var stdout, stderr string
		testutil.WithStdin(t, "not-json", func() {
			code, stdout, stderr = testutil.RunApp(t, app, "rewrite", "--hook", "gemini")
		})
		if code != 0 || stderr != "" || strings.TrimSpace(stdout) != `{"decision":"allow"}` {
			t.Fatalf("unexpected gemini fallback stdout=%q stderr=%q code=%d", stdout, stderr, code)
		}
	})
}

func TestDoctorShowsUpdateCheckStatusAndNotice(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.IntervalHours = 12
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), stubUpdater{
		report: updates.DoctorReport{
			Enabled:         true,
			Interval:        12 * time.Hour,
			Method:          updates.InstallMethodGo,
			UpgradeCommand:  "go install github.com/devr-tools/szr/cmd/szr@latest",
			LatestVersion:   "v0.2.0",
			CheckedAt:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
			UpdateAvailable: true,
		},
	})

	code, stdout, stderr := testutil.RunApp(t, app, "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"update checks: enabled",
		"update check interval: 12h0m0s",
		"install method: go-install",
		"latest stable: v0.2.0",
		"update available: yes",
		"upgrade command: go install github.com/devr-tools/szr/cmd/szr@latest",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected doctor stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "git", "status")
	if code != 0 {
		t.Fatalf("unexpected git status stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if stderr != "" {
		t.Fatalf("expected no inline update notice for non-interactive stderr, got stderr=%q", stderr)
	}
}

func TestSettingsInteractivePersistsConfig(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	app := cli.NewWithDependencies("test", cfg, paths, store, testutil.AppEngine(t, paths))

	var code int
	var stdout, stderr string
	testutil.WithStdin(t, "1\n1\n2\n1\n3\n12\n4\n2\n5\n20\n6\n11\n7\n2\n8\n2\n9\n2\n10\n1\n11\n2\n12\n2\nq\n", func() {
		code, stdout, stderr = testutil.RunApp(t, app, "settings")
	})
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected settings stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"settings",
		"current: disabled",
		"1. enable",
		"2. disable",
		"saved: update checks enabled",
		"saved: auto update enabled",
		"saved: update interval 12h",
		"saved: tee on failure disabled",
		"saved: max preview lines 20",
		"saved: max match groups 11",
		"saved: reasoning budget mode agent",
		"saved: aggressive prepare rewrites disabled",
		"saved: noise prefiltering disabled",
		"saved: adaptive budgets enabled",
		"saved: early capture stop disabled",
		"saved: semantic compaction disabled",
		"settings: saved and exiting",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected stdout to contain %q, got %q", want, stdout)
		}
	}

	data := testutil.MustReadFile(t, paths.ConfigFile)
	var saved config.Config
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if !saved.UpdateCheck.Enabled || !saved.UpdateCheck.AutoUpdate || saved.UpdateCheck.IntervalHours != 12 {
		t.Fatalf("unexpected update settings: %#v", saved.UpdateCheck)
	}
	if saved.TeeOnFailure || saved.MaxPreviewLines != 20 || saved.MaxMatchGroups != 11 || saved.ReasoningBudgetMode != config.ReasoningBudgetAgent {
		t.Fatalf("unexpected saved config: %#v", saved)
	}
	if saved.Advanced.AggressivePrepareRewrites || saved.Advanced.NoisePrefiltering || !saved.Advanced.AdaptiveBudgets || saved.Advanced.EarlyCaptureStop || saved.Advanced.SemanticCompaction {
		t.Fatalf("unexpected advanced settings: %#v", saved.Advanced)
	}
}

func TestSettingsRejectsUnknownArgs(t *testing.T) {
	app := testutil.NewTestApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "settings", "extra")
	if code != 2 || stdout != "" {
		t.Fatalf("unexpected settings args stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stderr, "settings does not accept arguments") {
		t.Fatalf("expected settings arg error, got %q", stderr)
	}
}

func TestDoctorJSONIncludesUpdateStatus(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.IntervalHours = 12
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), stubUpdater{
		report: updates.DoctorReport{
			Enabled:         true,
			Interval:        12 * time.Hour,
			Method:          updates.InstallMethodGo,
			UpgradeCommand:  "go install github.com/devr-tools/szr/cmd/szr@latest",
			LatestVersion:   "v0.2.0",
			LatestURL:       "https://github.com/devr-tools/szr/releases/tag/v0.2.0",
			CheckedAt:       time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
			UpdateAvailable: true,
		},
	})

	code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	var payload struct {
		Version string `json:"version"`
		Update  struct {
			Enabled         bool   `json:"enabled"`
			AutoUpdate      bool   `json:"auto_update"`
			InstallMethod   string `json:"install_method"`
			LatestVersion   string `json:"latest_version"`
			LatestURL       string `json:"latest_url"`
			CheckedAt       string `json:"checked_at"`
			UpdateAvailable bool   `json:"update_available"`
			UpgradeCommand  string `json:"upgrade_command"`
		} `json:"update"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode self doctor json: %v", err)
	}
	if payload.Version != "v0.1.0" {
		t.Fatalf("unexpected version %#v", payload)
	}
	if !payload.Update.Enabled || payload.Update.AutoUpdate || !payload.Update.UpdateAvailable {
		t.Fatalf("expected enabled available update, got %#v", payload.Update)
	}
	for _, want := range []string{
		payload.Update.InstallMethod,
		payload.Update.LatestVersion,
		payload.Update.LatestURL,
		payload.Update.CheckedAt,
		payload.Update.UpgradeCommand,
	} {
		if want == "" {
			t.Fatalf("expected populated update json, got %#v", payload.Update)
		}
	}
	if payload.Update.InstallMethod != "go-install" || payload.Update.LatestVersion != "v0.2.0" {
		t.Fatalf("unexpected update json %#v", payload.Update)
	}
}

func TestDoctorJSONUsesCachedUpdateStateWithRealUpdater(t *testing.T) {
	cases := []struct {
		name                string
		version             string
		latestVersion       string
		wantUpdateAvailable bool
	}{
		{
			name:                "update available from cached semver",
			version:             "v0.1.0",
			latestVersion:       "v0.2.0",
			wantUpdateAvailable: true,
		},
		{
			name:                "prerelease treated as same stable version",
			version:             "v0.2.0-rc1",
			latestVersion:       "v0.2.0",
			wantUpdateAvailable: false,
		},
		{
			name:                "invalid cached version suppresses update flag",
			version:             "v0.1.0",
			latestVersion:       "stable",
			wantUpdateAvailable: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paths := testutil.Paths(t.TempDir())
			testutil.EnsurePaths(t, paths)

			cfg := config.Default()
			cfg.UpdateCheck.Enabled = true
			cfg.UpdateCheck.IntervalHours = 24
			cfg.UpdateCheck.AutoUpdate = true

			now := time.Now().UTC().Truncate(time.Second)
			cache := map[string]any{
				"checked_at":                    now.Format(time.RFC3339),
				"latest_version":                tc.latestVersion,
				"latest_url":                    "https://example.com/releases/" + tc.latestVersion,
				"auto_update_attempted_at":      now.Add(-2 * time.Hour).Format(time.RFC3339),
				"auto_update_attempted_version": tc.latestVersion,
				"auto_update_succeeded_at":      now.Add(-time.Hour).Format(time.RFC3339),
				"auto_update_succeeded_version": tc.latestVersion,
			}
			data, err := json.Marshal(cache)
			if err != nil {
				t.Fatalf("marshal cache: %v", err)
			}
			if err := os.WriteFile(filepath.Join(paths.DataDir, "update-check.json"), append(data, '\n'), 0o644); err != nil {
				t.Fatalf("write cache: %v", err)
			}

			app := cli.NewWithDependencies(tc.version, cfg, paths, nil, testutil.AppEngine(t, paths))
			code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor", "--json")
			if code != 0 || stderr != "" {
				t.Fatalf("unexpected self doctor json stdout=%q stderr=%q code=%d", stdout, stderr, code)
			}

			var payload struct {
				Update struct {
					Enabled          bool   `json:"enabled"`
					AutoUpdate       bool   `json:"auto_update"`
					LatestVersion    string `json:"latest_version"`
					LatestURL        string `json:"latest_url"`
					FromCache        bool   `json:"from_cache"`
					UpdateAvailable  bool   `json:"update_available"`
					AttemptedAt      string `json:"attempted_at"`
					AttemptedVersion string `json:"attempted_version"`
					SucceededAt      string `json:"succeeded_at"`
					SucceededVersion string `json:"succeeded_version"`
				} `json:"update"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("decode self doctor json: %v", err)
			}
			if !payload.Update.Enabled || !payload.Update.AutoUpdate || !payload.Update.FromCache {
				t.Fatalf("expected cached enabled auto update payload, got %#v", payload.Update)
			}
			if payload.Update.LatestVersion != tc.latestVersion || payload.Update.LatestURL == "" {
				t.Fatalf("unexpected cached version payload %#v", payload.Update)
			}
			if payload.Update.UpdateAvailable != tc.wantUpdateAvailable {
				t.Fatalf("unexpected update availability %#v", payload.Update)
			}
			for _, want := range []string{
				payload.Update.AttemptedAt,
				payload.Update.AttemptedVersion,
				payload.Update.SucceededAt,
				payload.Update.SucceededVersion,
			} {
				if want == "" {
					t.Fatalf("expected populated auto update state, got %#v", payload.Update)
				}
			}
		})
	}
}

func TestRunTriggersAutoUpdateForNormalCommands(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.AutoUpdate = true
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\necho \"## main...origin/main\"\n")

	updater := &countingUpdater{}
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), updater)

	code, _, _ := testutil.RunApp(t, app, "git", "status")
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if updater.autoCalls != 1 {
		t.Fatalf("expected auto update once, got %d", updater.autoCalls)
	}

	code, _, _ = testutil.RunApp(t, app, "self", "doctor")
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if updater.autoCalls != 1 {
		t.Fatalf("expected self doctor to skip auto update, got %d", updater.autoCalls)
	}
}

type stubUpdater struct {
	report       updates.DoctorReport
	autoResult   updates.AutoUpdateResult
	updateResult updates.SelfUpdateResult
	updateErr    error
	updateStdout string
	autoCalls    int
}

func (s stubUpdater) Doctor(context.Context, string, config.UpdateCheck) updates.DoctorReport {
	return s.report
}

func (s stubUpdater) AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult {
	return s.autoResult
}

func (s stubUpdater) SelfUpdate(_ context.Context, stdout, _ io.Writer) (updates.SelfUpdateResult, error) {
	if s.updateStdout != "" {
		_, _ = io.WriteString(stdout, s.updateStdout)
	}
	return s.updateResult, s.updateErr
}

type countingUpdater struct {
	autoCalls int
}

func (u *countingUpdater) Doctor(context.Context, string, config.UpdateCheck) updates.DoctorReport {
	return updates.DoctorReport{}
}

func (u *countingUpdater) AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult {
	u.autoCalls++
	return updates.AutoUpdateResult{}
}

func (u *countingUpdater) SelfUpdate(context.Context, io.Writer, io.Writer) (updates.SelfUpdateResult, error) {
	return updates.SelfUpdateResult{}, nil
}
