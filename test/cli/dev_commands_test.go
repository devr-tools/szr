package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
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
	for _, key := range []string{"duration_p50_us", "duration_p95_us", "quality_score", "ok", "command_fingerprint"} {
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
	if len(payload) != 15 {
		t.Fatalf("expected all benchmark fixtures, got %#v", payload)
	}
}

func TestBenchMismatchOutput(t *testing.T) {
	app := testutil.NewTestApp(t)
	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")
	testutil.MustWriteFile(t, filepath.Join(root, ".szr"), "blocked")

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

func TestInstallCommands(t *testing.T) {
	app := testutil.NewTestApp(t)
	repo := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/demo\n")
	testutil.MustWriteFile(t, filepath.Join(repo, "cmd", "szr", "main.go"), "package main\n")

	restore := testutil.Chdir(t, repo)
	defer restore()

	code, stdout, stderr := testutil.RunApp(t, app, "install")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install list stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"available install targets:",
		"codex",
		"claude-code",
		"cursor",
		"gemini",
		"shell",
		"szr install --all",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install list stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "install", "--print", "cursor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install print stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: cursor", "./.cursor/rules/szr.mdc", "manual steps:"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install print stdout to contain %q, got %q", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".cursor", "rules", "szr.mdc")); !os.IsNotExist(err) {
		t.Fatalf("expected print mode not to write cursor rule, got err=%v", err)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "install", "codex")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install apply stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"installed codex", "./AGENTS.md", "./.szr/hooks/pre-command.sh"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install apply stdout to contain %q, got %q", want, stdout)
		}
	}
	for _, path := range []string{
		filepath.Join(repo, "AGENTS.md"),
		filepath.Join(repo, ".szr", "hooks", "pre-command.sh"),
		filepath.Join(repo, ".szr", "install", "codex.md"),
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
