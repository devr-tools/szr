package installers_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/installers"
	"github.com/devr-tools/szr/test/testutil"
)

func TestDetectPathsVariants(t *testing.T) {
	t.Run("prefers go run for source repo", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module github.com/devr-tools/szr\n")
		testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")

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
		t.Parallel()

		root := t.TempDir()
		testutil.MustWriteExecutable(t, filepath.Join(root, "bin", "szr"), "#!/bin/sh\n")

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "./bin/szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("falls back to repo executable", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		testutil.MustWriteExecutable(t, filepath.Join(root, "szr"), "#!/bin/sh\n")

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "./szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("falls back to shell binary name", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		paths, err := installers.DetectPaths(root)
		if err != nil {
			t.Fatalf("detect paths: %v", err)
		}
		if paths.Binary != "szr" {
			t.Fatalf("unexpected binary: %q", paths.Binary)
		}
	})

	t.Run("detects claude paths", func(t *testing.T) {
		t.Parallel()

		home := t.TempDir()

		paths, err := installers.DetectClaudeGlobalPaths(home)
		if err != nil {
			t.Fatalf("detect claude paths: %v", err)
		}
		if paths.Binary != "szr" {
			t.Fatalf("unexpected global binary: %q", paths.Binary)
		}
		if !strings.HasSuffix(paths.HookFile, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) {
			t.Fatalf("unexpected global hook path: %q", paths.HookFile)
		}
		if !strings.HasSuffix(paths.ClaudeConfig, filepath.Join(".claude", "settings.json")) {
			t.Fatalf("unexpected global settings path: %q", paths.ClaudeConfig)
		}
		if !strings.HasSuffix(paths.CodexSZRFile, filepath.Join(".codex", "szr.md")) {
			t.Fatalf("unexpected codex path: %q", paths.CodexSZRFile)
		}
		if !strings.HasSuffix(paths.CursorConfig, filepath.Join(".cursor", "hooks.json")) {
			t.Fatalf("unexpected cursor config path: %q", paths.CursorConfig)
		}
		if !strings.HasSuffix(paths.GeminiConfig, filepath.Join(".gemini", "settings.json")) {
			t.Fatalf("unexpected gemini config path: %q", paths.GeminiConfig)
		}
		if !paths.Global {
			t.Fatal("expected global paths to be marked global")
		}
	})

	t.Run("rejects invalid roots", func(t *testing.T) {
		t.Parallel()

		if _, err := installers.DetectPaths(""); err == nil {
			t.Fatal("expected empty root error")
		}

		root := t.TempDir()
		file := filepath.Join(root, "not-a-dir")
		testutil.MustWriteFile(t, file, "x")
		if _, err := installers.DetectPaths(file); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("expected directory error, got %v", err)
		}
	})
}
