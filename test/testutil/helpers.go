package testutil

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
)

func CaptureOutput(t *testing.T, fn func()) (string, string) {
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

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdoutBuf, stdoutR)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderrBuf, stderrR)
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	wg.Wait()
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return stdoutBuf.String(), stderrBuf.String()
}

func WithStdin(t *testing.T, input string, fn func()) {
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

func WriteExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if runtime.GOOS == "windows" {
		scriptName := name + ".sh"
		scriptPath := filepath.Join(dir, scriptName)
		if err := writeExecutableFile(scriptPath, []byte(body), 0o755); err != nil {
			t.Fatalf("write executable %s: %v", scriptName, err)
		}
		wrapperPath := filepath.Join(dir, name+".cmd")
		wrapper := "@echo off\r\n\"" + windowsBashPath() + "\" \"%~dp0" + scriptName + "\" %*\r\n"
		if err := writeExecutableFile(wrapperPath, []byte(wrapper), 0o755); err != nil {
			t.Fatalf("write wrapper %s: %v", filepath.Base(wrapperPath), err)
		}
		return wrapperPath
	}
	path := filepath.Join(dir, name)
	if err := writeExecutableFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", name, err)
	}
	return path
}

func MustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func MustWriteExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := writeExecutableFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeExecutableFile(path string, content []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, content, mode); err != nil {
		return err
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return nil
}

func MustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func Paths(root string) config.Paths {
	return config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
}

func EnsurePaths(t *testing.T, paths config.Paths) {
	t.Helper()
	if err := config.EnsurePaths(paths); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
}

func NewTestApp(t *testing.T) *cli.App {
	t.Helper()
	paths := Paths(t.TempDir())
	EnsurePaths(t, paths)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	return cli.NewWithDependencies("test", cfg, paths, store, eng)
}

func AppEngine(t *testing.T, paths config.Paths) *engine.Engine {
	t.Helper()
	cfg := config.Default()
	return engine.New(cfg, paths, history.New(paths.HistoryFile), profiles.Builtins(cfg.MaxPreviewLines))
}

func RunApp(t *testing.T, app *cli.App, args ...string) (int, string, string) {
	t.Helper()
	var code int
	stdout, stderr := CaptureOutput(t, func() {
		code = app.Run(context.Background(), args)
	})
	return code, stdout, stderr
}

func Chdir(t *testing.T, dir string) func() {
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

func FindProfile(t *testing.T, list []engine.Profile, name string) engine.Profile {
	t.Helper()
	for _, profile := range list {
		if profile.Name == name {
			return profile
		}
	}
	t.Fatalf("missing profile %s", name)
	return engine.Profile{}
}

func windowsBashPath() string {
	if runtime.GOOS != "windows" {
		return "bash"
	}
	if path, err := exec.LookPath("bash"); err == nil && path != "" {
		return path
	}
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Git", "bin", "bash.exe"),
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "bash"
}
