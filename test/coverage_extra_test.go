package test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/bench"
	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/internal/rules"
)

func TestBenchCoverageEdges(t *testing.T) {
	fixtures := bench.MustLoadFixtures(func() ([]bench.Fixture, error) {
		return []bench.Fixture{{Name: "ok"}}, nil
	})
	if len(fixtures) != 1 || fixtures[0].Name != "ok" {
		t.Fatalf("unexpected must load fixtures: %#v", fixtures)
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected MustLoadFixtures to panic on loader error")
		}
	}()
	_ = bench.MustLoadFixtures(func() ([]bench.Fixture, error) {
		return nil, errors.New("boom")
	})
}

func TestBenchLoadAndMeasureErrors(t *testing.T) {
	_, err := bench.LoadFixtures(func(name string) ([]byte, error) {
		if strings.Contains(name, "stdout") {
			return []byte("ok"), nil
		}
		return nil, errors.New("stderr boom")
	}, []bench.Spec{{
		Name:       "broken",
		StdoutFile: "testdata/stdout.txt",
		StderrFile: "testdata/stderr.txt",
	}})
	if err == nil || !strings.Contains(err.Error(), "stderr.txt") {
		t.Fatalf("expected stderr read error, got %v", err)
	}

	harness := bench.NewHarnessWithProfiles(nil)
	if _, err := harness.Measure(bench.Fixture{ProfileName: "missing"}); err == nil {
		t.Fatal("expected missing profile measure error")
	}
}

func TestConfigLoadWithEdgeErrors(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}

	_, _, err := config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return "", errors.New("cwd fail") },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if err == nil || !strings.Contains(err.Error(), "cwd fail") {
		t.Fatalf("expected getwd error, got %v", err)
	}

	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return root, nil },
		func(string) (os.FileInfo, error) { return nil, errors.New("stat fail") },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
	)
	if err == nil || !strings.Contains(err.Error(), "stat fail") {
		t.Fatalf("expected discover stat error, got %v", err)
	}

	projectRoot := filepath.Join(root, "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".szr.json"), []byte(`{"profiles":[{"name":"ok","match":{"command_prefix":["npm"]}}]}`), 0o644); err != nil {
		t.Fatalf("write project rule file: %v", err)
	}
	_, _, err = config.LoadWith(
		func() (config.Paths, error) { return paths, nil },
		func(config.Paths) error { return nil },
		func() (string, error) { return projectRoot, nil },
		os.Stat,
		func(name string) ([]byte, error) {
			if strings.HasSuffix(name, ".szr.json") {
				return nil, errors.New("project read fail")
			}
			return nil, os.ErrNotExist
		},
	)
	if err == nil || !strings.Contains(err.Error(), "project read fail") {
		t.Fatalf("expected project rule read error, got %v", err)
	}
}

func TestRuleCoverageEdges(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".szr.json"), []byte(`{"profiles":[{"name":"ok","match":{"command_prefix":["npm"]}}]}`), 0o644); err != nil {
		t.Fatalf("write rule file: %v", err)
	}
	path, format, err := rules.Discover(root)
	if err != nil || format != rules.FormatJSON || !strings.HasSuffix(path, ".szr.json") {
		t.Fatalf("unexpected discover result path=%q format=%q err=%v", path, format, err)
	}

	if _, err := rules.ParseFile(".szr.txt", []byte("{}")); err == nil || !strings.Contains(err.Error(), "unsupported project rule file format") {
		t.Fatalf("expected unsupported format error, got %v", err)
	}
	if _, err := rules.ParseJSON([]byte("{bad")); err == nil {
		t.Fatal("expected invalid json parse error")
	}
	if err := rules.Validate(rules.File{Profiles: []rules.Profile{{
		Name:  "rewrite-missing-args",
		Match: rules.Match{CommandPrefix: []string{"npm"}},
		Rewrite: rules.Rewrite{
			Mode: "append",
		},
	}}}); err == nil || !strings.Contains(err.Error(), "rewrite.args is required") {
		t.Fatalf("expected rewrite args validation error, got %v", err)
	}
	if err := rules.Validate(rules.File{Profiles: []rules.Profile{{
		Name:  "negative-lines",
		Match: rules.Match{CommandPrefix: []string{"npm"}},
		Render: rules.Render{
			Mode:     "compact",
			MaxLines: -1,
		},
	}}}); err == nil || !strings.Contains(err.Error(), "render.max_lines must be >= 0") {
		t.Fatalf("expected negative max lines error, got %v", err)
	}
}

func TestEngineRuleCoverageEdges(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "argvdump", "#!/bin/sh\nprintf '%s\\n' \"$@\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

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
	cfg.ProjectRules = rules.File{
		Profiles: []rules.Profile{
			{
				Name:        "replace-local",
				Description: "replace rule",
				Match: rules.Match{
					CommandPrefix: []string{"argvdump"},
					AllArgs:       []string{"--target"},
					AnyArgs:       []string{"--replace"},
					ExcludeArgs:   []string{"--skip"},
				},
				Rewrite: rules.Rewrite{
					Mode: "replace",
					Args: []string{"argvdump", "replaced", "--json"},
				},
				Render: rules.Render{
					Mode: "compact",
				},
			},
			{
				Name: "default-render",
				Match: rules.Match{
					CommandPrefix: []string{"argvdump"},
					AnyArgs:       []string{"--default-render"},
				},
			},
		},
	}

	e := engine.New(cfg, paths, history.New(paths.HistoryFile), nil)
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "--target", "--replace"},
		Display: []string{"argvdump", "--target", "--replace"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute replace rule: %v", err)
	}
	if !strings.Contains(result.Display, "replaced") || !strings.Contains(result.Display, "--json") {
		t.Fatalf("unexpected replaced display: %q", result.Display)
	}

	fallback := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "--target", "--replace", "--skip"},
		Display: []string{"argvdump", "--target", "--replace", "--skip"},
	})
	if fallback.Name != "passthrough" {
		t.Fatalf("expected excluded args to miss custom rule, got %#v", fallback)
	}

	alsoFallback := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "--replace"},
		Display: []string{"argvdump", "--replace"},
	})
	if alsoFallback.Name != "passthrough" {
		t.Fatalf("expected missing all_args to miss custom rule, got %#v", alsoFallback)
	}

	defaultRender, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "--default-render", "line1", "line2"},
		Display: []string{"argvdump", "--default-render", "line1", "line2"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute default render rule: %v", err)
	}
	if !strings.Contains(defaultRender.Display, "--default-render") {
		t.Fatalf("expected compact default render output, got %q", defaultRender.Display)
	}
}

func TestJSCoverageEdges(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n")
	mustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")
	if err := os.WriteFile(filepath.Join(root, ".szr"), []byte("blocked"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	restore := chdirTemp(t, root)
	defer restore()

	code, stdout, stderr := runApp(t, app, "install", "codex")
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
	if err := config.EnsurePaths(customPaths); err != nil {
		t.Fatalf("ensure custom paths: %v", err)
	}
	badBenchEngine := engine.New(config.Default(), customPaths, history.New(customPaths.HistoryFile), []engine.Profile{
		{
			Name: "go-test-json",
			Render: func(engine.Invocation, engine.Execution) string {
				return "wrong output"
			},
		},
	})
	badBenchApp := cli.NewWithDependencies("test", config.Default(), customPaths, history.New(customPaths.HistoryFile), badBenchEngine)
	code, stdout, stderr = runApp(t, badBenchApp, "bench", "clean-pass")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "ok=false") {
		t.Fatalf("unexpected bench mismatch stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestCLIInstallGetwdError(t *testing.T) {
	app := newTestApp(t)
	root := t.TempDir()
	restore := chdirTemp(t, root)
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove cwd: %v", err)
	}
	code, stdout, stderr := runApp(t, app, "install", "codex")
	restore()
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to resolve working directory") {
		t.Fatalf("unexpected getwd failure stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}
