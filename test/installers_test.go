package test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/installers"
)

func TestDetectPathsVariants(t *testing.T) {
	t.Run("prefers go run for source repo", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "go.mod"), "module szr\n")
		mustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "go run ./cmd/szr --" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
		if !strings.HasSuffix(paths.HookFile, filepath.Join(".szr", "hooks", "pre-command.sh")) {
			t.Fatalf("unexpected hook path: %q", paths.HookFile)
		}
	})

	t.Run("prefers local binary", func(t *testing.T) {
		root := t.TempDir()
		mustWriteExecutable(t, filepath.Join(root, "bin", "szr"), "#!/bin/sh\n")

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "./bin/szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("falls back to repo executable", func(t *testing.T) {
		root := t.TempDir()
		mustWriteExecutable(t, filepath.Join(root, "szr"), "#!/bin/sh\n")

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "./szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("falls back to shell binary name", func(t *testing.T) {
		root := t.TempDir()

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("rejects invalid roots", func(t *testing.T) {
		if _, err := installers.DetectPaths(""); err == nil {
			t.Fatal("expected empty root error")
		}

		root := t.TempDir()
		file := filepath.Join(root, "not-a-dir")
		mustWriteFile(t, file, "x")
		if _, err := installers.DetectPaths(file); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("expected directory error, got %v", err)
		}
	})
}

func TestRenderTargets(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module szr\n")
	mustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")

	plans, err := installers.RenderAll(installers.Options{
		RepoRoot: root,
		Binary:   "./dev/szr",
	})
	if err != nil {
		t.Fatalf("render all: %v", err)
	}
	if len(plans) != 4 {
		t.Fatalf("unexpected plan count: %d", len(plans))
	}

	for _, target := range installers.Targets() {
		plan, err := installers.Render(target, installers.Options{
			RepoRoot: root,
			Binary:   "./dev/szr",
		})
		if err != nil {
			t.Fatalf("render %s: %v", target, err)
		}
		if plan.Target != target || plan.Paths.Binary != "./dev/szr" {
			t.Fatalf("unexpected plan metadata: %#v", plan)
		}
		if len(plan.Files) != 3 {
			t.Fatalf("unexpected file count for %s: %d", target, len(plan.Files))
		}
		if len(plan.ManualSteps) != 2 {
			t.Fatalf("unexpected manual steps for %s: %v", target, plan.ManualSteps)
		}

		var sawHook, sawInstallDoc, sawInstruction bool
		for _, file := range plan.Files {
			switch {
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")):
				sawHook = true
				if file.Strategy != installers.StrategyWrite || file.Mode != 0o755 {
					t.Fatalf("unexpected hook file metadata: %#v", file)
				}
				if !strings.Contains(file.Content, "./dev/szr") || !strings.Contains(file.Content, "szr hint") {
					t.Fatalf("unexpected hook content: %q", file.Content)
				}
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "install", string(target)+".md")):
				sawInstallDoc = true
				if !strings.Contains(file.Content, "Hook command:") || !strings.Contains(file.Content, "./.szr/hooks/pre-command.sh") {
					t.Fatalf("unexpected install doc: %q", file.Content)
				}
			default:
				sawInstruction = true
				if target == installers.TargetCursor {
					if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "alwaysApply: true") {
						t.Fatalf("unexpected cursor file: %#v", file)
					}
				} else {
					if file.Strategy != installers.StrategyMerge || file.Marker == "" {
						t.Fatalf("unexpected merge file: %#v", file)
					}
					if !strings.Contains(file.Content, "explain <cmd...>") || !strings.Contains(file.Content, "proxy <cmd...>") {
						t.Fatalf("unexpected instruction body: %q", file.Content)
					}
				}
			}
		}
		if !sawHook || !sawInstallDoc || !sawInstruction {
			t.Fatalf("missing generated files for %s", target)
		}
	}

	if _, err := installers.Render("unknown", installers.Options{RepoRoot: root}); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestApplyPlanMergeAndIdempotence(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Existing\n")

	plan, err := installers.Render(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
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
	if !strings.Contains(content, "# Existing") {
		t.Fatalf("existing content lost: %q", content)
	}
	if strings.Count(content, "<!-- szr-codex:begin -->") != 1 {
		t.Fatalf("expected single codex block: %q", content)
	}
	if !strings.Contains(content, "## szr for Codex") || !strings.Contains(content, "./bin/szr proxy <cmd...>") {
		t.Fatalf("unexpected agents content: %q", content)
	}

	hookInfo, err := os.Stat(filepath.Join(root, ".szr", "hooks", "pre-command.sh"))
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if hookInfo.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: %v", hookInfo.Mode())
	}

	doc, err := os.ReadFile(filepath.Join(root, ".szr", "install", "codex.md"))
	if err != nil {
		t.Fatalf("read install doc: %v", err)
	}
	if !strings.Contains(string(doc), "Instruction file: ./AGENTS.md") {
		t.Fatalf("unexpected install doc: %q", string(doc))
	}
}

func TestApplyWithErrors(t *testing.T) {
	plan := installers.Plan{
		Files: []installers.File{
			{
				Path:     "/tmp/file.txt",
				Content:  "x",
				Mode:     0o644,
				Strategy: installers.StrategyWrite,
			},
		},
	}

	wantErr := errors.New("boom")
	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return wantErr },
		func(string) ([]byte, error) { return nil, nil },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected mkdir error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, wantErr },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected read error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return wantErr },
		func(string, os.FileMode) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected write error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return wantErr },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected chmod error, got %v", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustWriteExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
