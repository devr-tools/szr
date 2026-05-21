package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
)

func TestCLIBenchCommands(t *testing.T) {
	app := newTestApp(t)

	code, stdout, stderr := runApp(t, app, "bench", "clean-pass")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected bench output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"clean-pass", "profile=go-test-json", "ok=true"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected bench stdout to contain %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = runApp(t, app, "bench", "--json", "clean-pass")
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
}

func TestCLIInstallCommands(t *testing.T) {
	app := newTestApp(t)
	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "go.mod"), "module example.com/demo\n")
	mustWriteFile(t, filepath.Join(repo, "cmd", "szr", "main.go"), "package main\n")

	restore := chdirTemp(t, repo)
	defer restore()

	code, stdout, stderr := runApp(t, app, "install", "--print", "cursor")
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

	code, stdout, stderr = runApp(t, app, "install", "codex")
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

	code, stdout, stderr = runApp(t, app, "install", "--all", "--print")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected install all stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{"plan: codex", "plan: claude-code", "plan: cursor", "plan: gemini"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected install all stdout to contain %q, got %q", want, stdout)
		}
	}
}

func TestCLIDoctorProjectRules(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:       filepath.Join(root, "config"),
		ConfigFile:      filepath.Join(root, "config", "config.json"),
		DataDir:         filepath.Join(root, "data"),
		HistoryFile:     filepath.Join(root, "data", "history.jsonl"),
		TeeDir:          filepath.Join(root, "data", "tee"),
		ProjectRuleFile: filepath.Join(root, ".szr.json"),
	}
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	app := cli.NewWithDependencies("test", config.Default(), paths, nil, appEngineForCoverage(t, paths))

	code, stdout, stderr := runApp(t, app, "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "project rules: "+paths.ProjectRuleFile) {
		t.Fatalf("expected doctor stdout to include project rule file, got %q", stdout)
	}
}

func chdirTemp(t *testing.T, dir string) func() {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}
}
