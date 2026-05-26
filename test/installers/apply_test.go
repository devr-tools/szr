package installers_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/installers"
	"github.com/devr-tools/szr/test/testutil"
)

func TestApplyPlanMergeAndIdempotence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Existing\n")

	plan, err := installers.Render(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
		HomeDir:  home,
	})
	if err != nil {
		t.Fatalf("render codex: %v", err)
	}

	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply first pass: %v", err)
	}
	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply second pass: %v", err)
	}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read agents: %v", err)
	}
	content := string(agents)
	assertContainsAll(t, content, "# Existing", "@", "Use szr as the default wrapper")
	if strings.Count(content, "<!-- szr-codex:begin -->") != 1 {
		t.Fatalf("expected single codex block: %q", content)
	}

	assertFileMissing(t, filepath.Join(root, ".szr", "install", "codex.md"))
	doc, err := os.ReadFile(filepath.Join(home, ".codex", ".szr", "install", "codex.md"))
	if err != nil {
		t.Fatalf("read install doc: %v", err)
	}
	if !strings.Contains(string(doc), ".codex/szr.md") {
		t.Fatalf("unexpected install doc: %q", string(doc))
	}
	codexPath := filepath.Join(home, ".codex", "szr.md")
	if _, err := os.Stat(codexPath); err != nil {
		t.Fatalf("expected codex shared file: %v", err)
	}
	hookInfo, err := os.Stat(codexPath)
	if err != nil {
		t.Fatalf("stat codex shared file: %v", err)
	}
	if runtime.GOOS != "windows" && hookInfo.Mode()&0o111 != 0 {
		t.Fatalf("expected codex shared file to be non-executable: %v", hookInfo.Mode())
	}
}

func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("expected %s to be absent, got %q", path, string(data))
	}
}

func TestApplyWithErrors(t *testing.T) {
	t.Parallel()

	plan := installers.Plan{
		Files: []installers.File{{
			Path:     "/tmp/file.txt",
			Content:  "x",
			Mode:     0o644,
			Strategy: installers.StrategyWrite,
		}},
	}

	wantErr := errors.New("boom")
	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return wantErr },
		func(string) ([]byte, error) { return nil, nil },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected mkdir error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, wantErr },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected read error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return wantErr },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected write error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return wantErr },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected chmod error, got %v", err)
	}
}

func TestApplyEmptyMergePlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan := installers.Plan{
		Files: []installers.File{{
			Path:     filepath.Join(root, "AGENTS.md"),
			Content:  "",
			Mode:     0o644,
			Strategy: installers.StrategyMerge,
			Marker:   "szr-empty",
		}},
	}
	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply empty merge plan: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read empty merge file: %v", err)
	}
	if !strings.Contains(string(data), "<!-- szr-empty:begin -->") {
		t.Fatalf("expected empty merge markers, got %q", string(data))
	}
}

func TestApplyUninstallPlanRemovesManagedContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Existing\n")

	installPlan, err := installers.Render(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
		HomeDir:  home,
	})
	if err != nil {
		t.Fatalf("render install plan: %v", err)
	}
	if err := installers.Apply(installPlan); err != nil {
		t.Fatalf("apply install plan: %v", err)
	}

	uninstallPlan, err := installers.RenderUninstall(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
		HomeDir:  home,
	})
	if err != nil {
		t.Fatalf("render uninstall plan: %v", err)
	}
	if err := installers.Apply(uninstallPlan); err != nil {
		t.Fatalf("apply uninstall plan: %v", err)
	}

	agents := string(testutil.MustReadFile(t, filepath.Join(root, "AGENTS.md")))
	if agents != "# Existing\n" {
		t.Fatalf("expected original AGENTS content to survive, got %q", agents)
	}
	for _, path := range []string{
		filepath.Join(home, ".codex", ".szr", "install", "codex.md"),
		filepath.Join(home, ".codex", "szr.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", path, err)
		}
	}
}

func TestApplyClaudePlanAndUninstall(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# Existing\n")
	testutil.MustWriteFile(t, filepath.Join(home, ".claude", "settings.json"), "{\n  \"theme\": \"dark\"\n}\n")

	installPlan, err := installers.Render(installers.TargetClaude, installers.Options{
		HomeDir: home,
		Binary:  "szr",
	})
	if err != nil {
		t.Fatalf("render claude install plan: %v", err)
	}
	if err := installers.Apply(installPlan); err != nil {
		t.Fatalf("apply claude install plan: %v", err)
	}

	claudeMD := string(testutil.MustReadFile(t, filepath.Join(home, ".claude", "CLAUDE.md")))
	if !strings.Contains(claudeMD, "# Existing") || !strings.Contains(claudeMD, "@szr.md") {
		t.Fatalf("unexpected global CLAUDE.md content: %q", claudeMD)
	}
	settings := string(testutil.MustReadFile(t, filepath.Join(home, ".claude", "settings.json")))
	if !strings.Contains(settings, "\"theme\": \"dark\"") {
		t.Fatalf("unexpected global settings.json content: %q", settings)
	}
	assertClaudeHookCommand(t, settings, filepath.Join(home, ".claude", "hooks", "szr-rewrite.sh"))
	if _, err := os.Stat(filepath.Join(home, ".claude", "szr.md")); err != nil {
		t.Fatalf("expected szr.md to exist: %v", err)
	}

	uninstallPlan, err := installers.RenderUninstall(installers.TargetClaude, installers.Options{
		HomeDir: home,
		Binary:  "szr",
	})
	if err != nil {
		t.Fatalf("render claude uninstall plan: %v", err)
	}
	if err := installers.Apply(uninstallPlan); err != nil {
		t.Fatalf("apply claude uninstall plan: %v", err)
	}

	claudeMD = string(testutil.MustReadFile(t, filepath.Join(home, ".claude", "CLAUDE.md")))
	if claudeMD != "# Existing\n" {
		t.Fatalf("expected original global CLAUDE.md content to survive, got %q", claudeMD)
	}
	settings = string(testutil.MustReadFile(t, filepath.Join(home, ".claude", "settings.json")))
	if strings.Contains(settings, "szr-rewrite.sh") || !strings.Contains(settings, "\"theme\": \"dark\"") {
		t.Fatalf("unexpected global settings.json after uninstall: %q", settings)
	}
	for _, path := range []string{
		filepath.Join(home, ".claude", "hooks", "szr-rewrite.sh"),
		filepath.Join(home, ".claude", ".szr", "install", "claude-code.md"),
		filepath.Join(home, ".claude", "szr.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", path, err)
		}
	}
}

func assertClaudeHookCommand(t *testing.T, content, want string) {
	t.Helper()

	var payload struct {
		Hooks struct {
			PreToolUse []struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("decode Claude settings.json: %v", err)
	}
	for _, entry := range payload.Hooks.PreToolUse {
		for _, hook := range entry.Hooks {
			if hook.Command == want {
				return
			}
		}
	}
	t.Fatalf("expected Claude hook command %q in settings.json, got %q", want, content)
}

func TestApplyCursorAndGeminiPlansAndUninstall(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(home, ".cursor", "hooks.json"), "{\n  \"theme\": \"dark\"\n}\n")
	testutil.MustWriteFile(t, filepath.Join(home, ".gemini", "settings.json"), "{\n  \"theme\": \"dark\"\n}\n")

	for _, target := range []installers.Target{installers.TargetCursor, installers.TargetGemini} {
		installPlan, err := installers.Render(target, installers.Options{
			RepoRoot: home,
			HomeDir:  home,
			Binary:   "szr",
		})
		if err != nil {
			t.Fatalf("render %s install plan: %v", target, err)
		}
		if err := installers.Apply(installPlan); err != nil {
			t.Fatalf("apply %s install plan: %v", target, err)
		}

		uninstallPlan, err := installers.RenderUninstall(target, installers.Options{
			RepoRoot: home,
			HomeDir:  home,
			Binary:   "szr",
		})
		if err != nil {
			t.Fatalf("render %s uninstall plan: %v", target, err)
		}
		if err := installers.Apply(uninstallPlan); err != nil {
			t.Fatalf("apply %s uninstall plan: %v", target, err)
		}
	}

	cursorSettings := string(testutil.MustReadFile(t, filepath.Join(home, ".cursor", "hooks.json")))
	if strings.Contains(cursorSettings, "szr-rewrite.sh") || !strings.Contains(cursorSettings, "\"theme\": \"dark\"") {
		t.Fatalf("unexpected cursor hooks.json content: %q", cursorSettings)
	}
	geminiSettings := string(testutil.MustReadFile(t, filepath.Join(home, ".gemini", "settings.json")))
	if strings.Contains(geminiSettings, "szr-rewrite.sh") || !strings.Contains(geminiSettings, "\"theme\": \"dark\"") {
		t.Fatalf("unexpected gemini settings.json content: %q", geminiSettings)
	}
}
