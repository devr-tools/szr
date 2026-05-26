package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/updates"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBenchCommands(t *testing.T) {
	app := testutil.NewTestApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "bench", "clean-pass")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected bench output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"clean-pass", "profile=go-test-json", "dur_p50=", "quality=", "ok=true"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected bench stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, want := range []string{"lines=", "summary fixtures=", "hotspots="} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected bench stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "bench", "--json", "clean-pass")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected bench json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode bench json: %v", err)
	}
	if len(payload) != 1 || payload[0]["fixture_name"] != "clean-pass" {
		t.Fatalf("unexpected bench json payload: %#v", payload)
	}
	for _, key := range []string{"duration_p50_us", "duration_p95_us", "quality_score", "ok", "command_fingerprint", "raw_lines", "reduced_lines", "raw_preview", "reduced_preview"} {
		if _, ok := payload[0][key]; !ok {
			t.Fatalf("expected bench json key %q in payload %#v", key, payload[0])
		}
	}
}

func TestBenchCoverageEdges(t *testing.T) {
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), engine.New(config.Default(), paths, history.New(paths.HistoryFile), nil))
	code, stdout, stderr := testutil.RunApp(t, app, "bench", "clean-pass")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to benchmark clean-pass") {
		t.Fatalf("unexpected bench failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	okApp := testutil.NewTestApp(t)
	code, stdout, stderr = testutil.RunApp(t, okApp, "bench", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected bench all json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode bench all json: %v", err)
	}
	if len(payload) != 17 {
		t.Fatalf("expected all benchmark fixtures, got %#v", payload)
	}
}

func TestBenchMismatchOutput(t *testing.T) {
	app := testutil.NewTestApp(t)
	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")
	testutil.MustWriteFile(t, filepath.Join(root, ".szr"), "blocked")
	t.Setenv("CODEX_HOME", filepath.Join(root, ".szr"))

	restore := testutil.Chdir(t, root)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "install", "codex")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to install codex") {
		t.Fatalf("unexpected install failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	customPaths := config.Paths{
		ConfigDir:   filepath.Join(root, "cfg"),
		ConfigFile:  filepath.Join(root, "cfg", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
	testutil.EnsurePaths(t, customPaths)
	badBenchEngine := engine.New(config.Default(), customPaths, history.New(customPaths.HistoryFile), []engine.Profile{{
		Name: "go-test-json",
		Render: func(engine.Invocation, engine.Execution) string {
			return "wrong output"
		},
	}})
	badBenchApp := cli.NewWithDependencies("test", config.Default(), customPaths, history.New(customPaths.HistoryFile), badBenchEngine)
	code, stdout, stderr = testutil.RunApp(t, badBenchApp, "bench", "clean-pass")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "ok=false") {
		t.Fatalf("unexpected bench mismatch stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestInstallListAndPrint(t *testing.T) {
	app, repo, restore := newInstallCommandsFixture(t)
	defer restore()
	code, stdout, stderr := testutil.RunApp(t, app, "install")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install list stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"available install targets:", "codex", "claude-code", "cursor", "gemini", "shell", "szr install --all"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install list stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "install", "--print", "cursor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: cursor", "hooks.json", "manual steps:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install print stdout to contain %q, got %q", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("expected print mode not to write cursor hooks, got err=%v", err)
	}
}

func TestInstallApplyAndAllPlans(t *testing.T) {
	app, repo, restore := newInstallCommandsFixture(t)
	defer restore()
	code, stdout, stderr := testutil.RunApp(t, app, "install", "codex")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"installed codex", "AGENTS.md", ".codex/szr.md"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install apply stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, path := range []string{
		filepath.Join(repo, "AGENTS.md"),
		filepath.Join(os.Getenv("HOME"), ".codex", "szr.md"),
		filepath.Join(os.Getenv("HOME"), ".codex", ".szr", "install", "codex.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed file %s: %v", path, err)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "install", "--all", "--print")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install all stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: codex", "plan: claude-code", "plan: cursor", "plan: gemini", "plan: shell"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install all stdout to contain %q, got %q", want, stdout)
		}
	}
}

func TestInstallClaudeGlobalPrintAndApply(t *testing.T) {
	app := testutil.NewTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	code, stdout, stderr := testutil.RunApp(t, app, "install", "--print", "claude-code")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected global install print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: claude-code", "./.claude/szr.md", "./.claude/settings.json"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected global install print stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "install", "claude-code")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected global install apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"installed claude-code", "./.claude/szr.md", "./.claude/settings.json"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected global install apply stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, path := range []string{
		filepath.Join(home, ".claude", "szr.md"),
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "hooks", "szr-rewrite.sh"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed global file %s: %v", path, err)
		}
	}
}

func TestUninstallPrintRepoTarget(t *testing.T) {
	app, repo, restore := newInstallCommandsFixture(t)
	defer restore()
	_, _, _ = testutil.RunApp(t, app, "install", "codex")

	code, stdout, stderr := testutil.RunApp(t, app, "uninstall", "--print", "codex")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected uninstall print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: uninstall codex", "AGENTS.md", ".codex/szr.md", "manual steps:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected uninstall print stdout to contain %q, got %q", want, stdout)
		}
	}
	_ = repo
}

func TestUninstallApplyRemovesFiles(t *testing.T) {
	app, repo, restore := newInstallCommandsFixture(t)
	defer restore()
	_, _, _ = testutil.RunApp(t, app, "install", "codex")
	code, stdout, stderr := testutil.RunApp(t, app, "uninstall", "codex")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected uninstall apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"uninstalled codex", "AGENTS.md", ".codex/szr.md"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected uninstall apply stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, path := range []string{
		filepath.Join(os.Getenv("HOME"), ".codex", ".szr", "install", "codex.md"),
		filepath.Join(os.Getenv("HOME"), ".codex", "szr.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected uninstalled file %s to be gone: %v", path, err)
		}
	}
	agentsPath := filepath.Join(repo, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		if strings.Contains(string(data), "Use szr as the default wrapper") {
			t.Fatalf("expected Codex instructions removed, got %q", string(data))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("read %s: %v", agentsPath, err)
	}
}

func TestUninstallClaudeGlobalPrintAndApply(t *testing.T) {
	app := testutil.NewTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, _, _ = testutil.RunApp(t, app, "install", "claude-code")

	code, stdout, stderr := testutil.RunApp(t, app, "uninstall", "--print", "claude-code")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected global uninstall print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: uninstall claude-code", "./.claude/szr.md", "./.claude/settings.json"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected global uninstall print stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "uninstall", "claude-code")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected global uninstall apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"uninstalled claude-code", "./.claude/szr.md", "./.claude/settings.json"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected global uninstall apply stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, path := range []string{
		filepath.Join(home, ".claude", "szr.md"),
		filepath.Join(home, ".claude", "settings.json"),
		filepath.Join(home, ".claude", "hooks", "szr-rewrite.sh"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected uninstalled global file %s to be gone: %v", path, err)
		}
	}
}

func TestInstallAllAndUninstallAllPrint(t *testing.T) {
	app, _, restore := newInstallCommandsFixture(t)
	defer restore()
	code, stdout, stderr := testutil.RunApp(t, app, "install", "--all")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install all apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "uninstall", "--all", "--print")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected uninstall all stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: uninstall codex", "plan: uninstall claude-code", "plan: uninstall cursor", "plan: uninstall gemini", "plan: uninstall shell"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected uninstall all stdout to contain %q, got %q", want, stdout)
		}
	}
}

func TestInstallClaudeGlobalFlagRejected(t *testing.T) {
	app := testutil.NewTestApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "install", "--global", "claude-code")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown install flag --global") {
		t.Fatalf("unexpected global install flag handling stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "uninstall", "--global", "claude-code")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown uninstall flag --global") {
		t.Fatalf("unexpected global uninstall flag handling stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func newInstallCommandsFixture(t *testing.T) (*cli.App, string, func()) {
	t.Helper()
	app := testutil.NewTestApp(t)
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	testutil.MustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/demo\n")
	testutil.MustWriteFile(t, filepath.Join(repo, "cmd", "szr", "main.go"), "package main\n")
	restore := testutil.Chdir(t, repo)
	return app, repo, restore
}

func TestSelfInstallCommands(t *testing.T) {
	app := testutil.NewTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin")

	binaryName := "szr"
	if runtime.GOOS == "windows" {
		binaryName = "szr.exe"
	}

	code, stdout, stderr := testutil.RunApp(t, app, "self", "install", "--print")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self install print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"plan: self install",
		filepath.Join(home, ".local", "bin", binaryName),
		"path: missing",
		`export PATH="$HOME/.local/bin:$PATH"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected self install print stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "self", "install")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self install stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	target := filepath.Join(home, ".local", "bin", binaryName)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected installed szr binary at %s: %v", target, err)
	}
	for _, want := range []string{
		"installed: " + target,
		"path: missing",
		"shell rc: " + filepath.Join(home, ".zshrc"),
		`export PATH="$HOME/.local/bin:$PATH"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected self install stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "uninstall", "--print")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self uninstall print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"plan: self uninstall",
		target,
		filepath.Join(home, ".zshrc"),
		`export PATH="$HOME/.local/bin:$PATH"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected self uninstall print stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "uninstall")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self uninstall stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected uninstalled szr binary at %s to be gone: %v", target, err)
	}
	if !strings.Contains(stdout, "uninstalled: "+target) {
		t.Fatalf("expected self uninstall stdout to contain target, got %q", stdout)
	}
}

func TestSelfInstallUpdateShell(t *testing.T) {
	app := testutil.NewTestApp(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", "/usr/bin")

	code, stdout, stderr := testutil.RunApp(t, app, "self", "install", "--path", filepath.Join(home, "bin"), "--update-shell")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self install update stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "shell rc updated: yes") {
		t.Fatalf("expected shell rc update in stdout, got %q", stdout)
	}
	rcPath := filepath.Join(home, ".bashrc")
	content := string(testutil.MustReadFile(t, rcPath))
	if !strings.Contains(content, `export PATH="$HOME/bin:$PATH"`) {
		t.Fatalf("expected bashrc to contain PATH export, got %q", content)
	}
}

func TestSelfUpdateCommand(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths), stubUpdater{
		updateResult: updates.SelfUpdateResult{
			Method:         updates.InstallMethodBrew,
			UpgradeCommand: "brew upgrade szr",
		},
		updateStdout: "brew updated\n",
	})

	code, stdout, stderr := testutil.RunApp(t, app, "self", "update")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self update stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"brew updated", "updated via: brew", "command: brew upgrade szr"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected self update stdout to contain %q, got %q", want, stdout)
		}
	}

	app = cli.NewWithDependenciesAndUpdater("v0.1.0", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths), stubUpdater{
		updateResult: updates.SelfUpdateResult{},
		updateErr:    errors.New("unable to determine how this szr binary was installed"),
	})
	code, stdout, stderr = testutil.RunApp(t, app, "self", "update")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "unable to determine how this szr binary was installed") {
		t.Fatalf("unexpected self update failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestInstallGetwdError(t *testing.T) {
	app := testutil.NewTestApp(t)
	root := t.TempDir()
	restore := testutil.Chdir(t, root)
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}
	code, stdout, stderr := testutil.RunApp(t, app, "install", "codex")
	restore()
	if code != 2 || stdout != "" || !strings.Contains(stderr, "no such file or directory") {
		t.Fatalf("unexpected deleted-cwd install failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	restore = testutil.Chdir(t, t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd after chdir: %v", err)
	}
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatalf("remove second cwd: %v", err)
	}
	code, stdout, stderr = testutil.RunApp(t, app, "install", "--all")
	restore()
	if code != 1 || stdout != "" || !strings.Contains(stderr, "no such file or directory") {
		t.Fatalf("unexpected deleted-cwd install all failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	restore = testutil.Chdir(t, t.TempDir())
	cwd, err = os.Getwd()
	if err != nil {
		t.Fatalf("getwd before uninstall: %v", err)
	}
	if err := os.RemoveAll(cwd); err != nil {
		t.Fatalf("remove third cwd: %v", err)
	}
	code, stdout, stderr = testutil.RunApp(t, app, "uninstall", "codex")
	restore()
	if code != 2 || stdout != "" || !strings.Contains(stderr, "no such file or directory") {
		t.Fatalf("unexpected deleted-cwd uninstall failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestDoctorProjectRules(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:       filepath.Join(root, "config"),
		ConfigFile:      filepath.Join(root, "config", "config.json"),
		DataDir:         filepath.Join(root, "data"),
		HistoryFile:     filepath.Join(root, "data", "history.jsonl"),
		TeeDir:          filepath.Join(root, "data", "tee"),
		ProjectRuleFile: filepath.Join(root, ".szr.json"),
	}
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependencies("test", config.Default(), paths, nil, testutil.AppEngine(t, paths))

	code, stdout, stderr := testutil.RunApp(t, app, "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "project rules: "+paths.ProjectRuleFile) {
		t.Fatalf("expected doctor stdout to include project rule file, got %q", stdout)
	}
}
