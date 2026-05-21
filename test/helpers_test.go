package test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/internal/profiles"
)

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return stdoutBuf.String(), stderrBuf.String()
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := writer.WriteString(input); err != nil {
		t.Fatalf("stdin write: %v", err)
	}
	_ = writer.Close()
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}()

	fn()
}

func writeExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
	return path
}

func newTestApp(t *testing.T) *cli.App {
	t.Helper()
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	return cli.NewWithDependencies("test", cfg, paths, store, eng)
}

func appEngineForCoverage(t *testing.T, paths config.Paths) *engine.Engine {
	t.Helper()
	cfg := config.Default()
	return engine.New(cfg, paths, history.New(paths.HistoryFile), profiles.Builtins(cfg.MaxPreviewLines))
}

func runApp(t *testing.T, app *cli.App, args ...string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = app.Run(context.Background(), args)
	})
	return code, stdout, stderr
}
