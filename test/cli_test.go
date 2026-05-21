package test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/history"
)

func TestCLIConstructors(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if app := cli.New("1.2.3"); app == nil {
		t.Fatal("expected cli.New app")
	}

	app := cli.NewWithLoader(
		"1.2.3",
		func() (config.Config, config.Paths, error) {
			root := t.TempDir()
			paths := config.Paths{
				ConfigDir:   filepath.Join(root, "config"),
				ConfigFile:  filepath.Join(root, "config", "config.json"),
				DataDir:     filepath.Join(root, "data"),
				HistoryFile: filepath.Join(root, "data", "history.jsonl"),
				TeeDir:      filepath.Join(root, "data", "tee"),
			}
			return config.Default(), paths, nil
		},
		func(int) {},
	)
	if app == nil {
		t.Fatal("expected loaded app")
	}

	stdout, stderr := captureOutput(t, func() {
		got := cli.NewWithLoader(
			"1.2.3",
			func() (config.Config, config.Paths, error) {
				return config.Config{}, config.Paths{}, errors.New("boom")
			},
			func(int) {},
		)
		if got != nil {
			t.Fatal("expected nil app on load error")
		}
	})
	if stdout != "" || !strings.Contains(stderr, "failed to load config: boom") {
		t.Fatalf("unexpected constructor output stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestCLIRunRoutes(t *testing.T) {
	app := newTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeExecutable(t, binDir, "git", `#!/bin/sh
case "$1" in
status)
  echo "## main...origin/main"
  echo "M  README.md"
  ;;
log)
  echo "abc123 first"
  echo "def456 second"
  ;;
diff)
  echo "diff --git a/a.go b/a.go"
  echo " a.go | 2 +-"
  echo " 1 file changed, 1 insertion(+), 1 deletion(-)"
  ;;
esac
`)
	writeExecutable(t, binDir, "go", `#!/bin/sh
case "$1" in
test)
  echo '{"Action":"pass","Package":"pkg/pass"}'
  echo '{"Action":"fail","Package":"pkg/fail"}'
  echo '{"Action":"fail","Package":"pkg/fail","Test":"TestSad"}'
  ;;
build)
  echo 'compile error' >&2
  exit 1
  ;;
vet)
  echo 'warning: suspicious' >&2
  exit 1
  ;;
esac
`)
	writeExecutable(t, binDir, "echoer", "#!/bin/sh\necho plain-output\n")
	writeExecutable(t, binDir, "noisy", "#!/bin/sh\necho FAIL one\necho note >&2\n")
	writeExecutable(t, binDir, "rg", `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "__error__" ]; then
    echo "bad rg" >&2
    exit 2
  fi
  if [ "$arg" = "nomatch" ]; then
    exit 1
  fi
done
if [ "$1" = "__missing__" ]; then
  exit 1
fi
echo "file.go:12:match one"
echo "file.go:20:match two"
`)

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
	if err := os.MkdirAll(filepath.Join(root, "dir", "sub"), 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dir", "sub", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir deep tree: %v", err)
	}

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
		stdin      string
	}{
		{"help empty", nil, 0, []string{`szr: "sizer"`}, nil, ""},
		{"help flag", []string{"--help"}, 0, []string{"Core commands:"}, nil, ""},
		{"help ultra", []string{"-u", "help"}, 0, []string{"Core commands:"}, nil, ""},
		{"help verbose long", []string{"--verbose", "help"}, 0, []string{"Core commands:"}, nil, ""},
		{"help verbose exact", []string{"-vv", "help"}, 0, []string{"Core commands:"}, nil, ""},
		{"help verbose counted", []string{"-vvvv", "help"}, 0, []string{"Core commands:"}, nil, ""},
		{"version", []string{"--version"}, 0, []string{"szr test"}, nil, ""},
		{"profiles", []string{"profiles"}, 0, []string{"git-status", "generic-summary"}, nil, ""},
		{"doctor", []string{"doctor"}, 0, []string{"version: test", "go:", "git:", "rg:"}, nil, ""},
		{"doctor missing tool", []string{"doctor"}, 0, []string{"go: missing"}, nil, ""},
		{"git status", []string{"git", "status"}, 0, []string{"staged=1"}, nil, ""},
		{"git log", []string{"git", "log"}, 0, []string{"2 commits"}, nil, ""},
		{"git diff", []string{"git", "diff"}, 0, []string{"files=1 +0 -0", "a.go | 2 +-"}, nil, ""},
		{"go test", []string{"go", "test", "./..."}, 0, []string{"pkg/fail", "TestSad"}, nil, ""},
		{"go build", []string{"go", "build"}, 1, []string{"compile error"}, nil, ""},
		{"go vet", []string{"go", "vet"}, 1, []string{"warning: suspicious"}, nil, ""},
		{"run default route", []string{"echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"run explicit", []string{"run", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"proxy", []string{"proxy", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"test wrapper", []string{"test", "noisy"}, 0, []string{"FAIL one"}, nil, ""},
		{"summary wrapper", []string{"summary", "echoer"}, 0, []string{"plain-output"}, nil, ""},
		{"explain", []string{"explain", "git", "status"}, 0, []string{"profile: git-status"}, nil, ""},
		{"ls", []string{"ls", root}, 0, []string{filepath.Base(root), "dir", "sub"}, nil, ""},
		{"ls default root", []string{"ls"}, 0, []string{filepath.Base(".")}, nil, ""},
		{"read single", []string{"read", fileA}, 0, []string{"one", "// c"}, nil, ""},
		{"read multi aggressive", []string{"read", "-l", "aggressive", "-n", "--max-lines", "1", fileA, fileB}, 0, []string{"== " + fileA + " ==", "== " + fileB + " ==", "func x() { ... }"}, nil, ""},
		{"grep", []string{"grep", "match", "."}, 0, []string{"file.go (2 matches)"}, nil, ""},
		{"grep default path", []string{"grep", "match"}, 0, []string{"file.go (2 matches)"}, nil, ""},
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
				code, stdout, stderr = runApp(t, app, tc.args...)
			}
			if tc.stdin != "" {
				withStdin(t, tc.stdin, run)
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

func TestCLIErrorsAndSpread(t *testing.T) {
	app := newTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	writeExecutable(t, binDir, "rg", `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "__error__" ]; then
    echo "bad rg" >&2
    exit 2
  fi
  if [ "$arg" = "nomatch" ]; then
    exit 1
  fi
done
echo "file.go:12:match one"
`)

	root := t.TempDir()
	file := filepath.Join(root, "one.txt")
	if err := os.WriteFile(file, []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	errorCases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{"proxy missing", []string{"proxy"}, 2, nil, []string{"missing command for proxy"}},
		{"explain missing", []string{"explain"}, 2, nil, []string{"explain requires a command"}},
		{"spread bad flag", []string{"spread", "--bad"}, 2, nil, []string{"unknown spread flag"}},
		{"install missing", []string{"install"}, 2, nil, []string{"install requires a target or --all"}},
		{"install mixed targets", []string{"install", "--all", "codex"}, 2, nil, []string{"either --all or explicit targets"}},
		{"install bad flag", []string{"install", "--bad"}, 2, nil, []string{"unknown install flag"}},
		{"bench bad flag", []string{"bench", "--bad"}, 2, nil, []string{"unknown bench flag"}},
		{"bench no fixtures", []string{"bench", "missing-fixture"}, 2, nil, []string{"no benchmark fixtures matched"}},
		{"read missing", []string{"read"}, 2, nil, []string{"read requires a file"}},
		{"read missing level", []string{"read", "-l"}, 2, nil, []string{"missing value for --level"}},
		{"read missing max-lines", []string{"read", "--max-lines"}, 2, nil, []string{"missing value for --max-lines"}},
		{"read file error", []string{"read", filepath.Join(root, "missing.txt")}, 1, nil, []string{"no such file"}},
		{"grep missing", []string{"grep"}, 2, nil, []string{"grep requires a pattern"}},
		{"grep missing rg", []string{"grep", "match", "."}, 1, nil, []string{"executable file not found"}},
		{"grep error", []string{"grep", "pattern", ".", "__error__"}, 2, nil, []string{"bad rg"}},
		{"json missing args", []string{"json"}, 2, nil, []string{"json requires a file"}},
		{"json read error", []string{"json", filepath.Join(root, "missing.json")}, 1, nil, []string{"no such file"}},
		{"log read error", []string{"log", filepath.Join(root, "missing.log")}, 1, nil, []string{"no such file"}},
		{"ls error", []string{"ls", filepath.Join(root, "missing-dir")}, 1, nil, []string{"no such file"}},
		{"run exec error", []string{"run", "missing-binary"}, 1, nil, []string{"executable file not found"}},
		{"grep no match", []string{"grep", "nomatch", "."}, 0, []string{"no matches"}, nil},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "grep missing rg" {
				t.Setenv("PATH", t.TempDir())
			}
			code, stdout, stderr := runApp(t, app, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("unexpected code %d want %d stdout=%q stderr=%q", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout %q in %q", want, stdout)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr %q in %q", want, stderr)
				}
			}
		})
	}

	code, stdout, stderr := runApp(t, app, "spread")
	if code != 0 || !strings.Contains(stdout, "commands:") || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = runApp(t, app, "spread", "--history")
	if code != 0 || !strings.Contains(stdout, "recent:") {
		t.Fatalf("unexpected spread history output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = runApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil || payload.Commands == 0 {
		t.Fatalf("unexpected spread json payload: %#v err=%v", payload, err)
	}

	emptyApp := newTestApp(t)
	code, stdout, stderr = runApp(t, emptyApp, "spread")
	if code != 0 || strings.TrimSpace(stdout) != "no tracked commands yet" || stderr != "" {
		t.Fatalf("unexpected empty spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	badRoot := t.TempDir()
	badPaths := config.Paths{
		ConfigDir:   filepath.Join(badRoot, "config"),
		ConfigFile:  filepath.Join(badRoot, "config", "config.json"),
		DataDir:     filepath.Join(badRoot, "data"),
		HistoryFile: badRoot,
		TeeDir:      filepath.Join(badRoot, "tee"),
	}
	if err := config.EnsurePaths(badPaths); err != nil {
		t.Fatalf("ensure bad paths: %v", err)
	}
	badStore := history.New(badRoot)
	badApp := cli.NewWithDependencies("test", config.Default(), badPaths, badStore, appEngineForCoverage(t, badPaths))
	code, stdout, stderr = runApp(t, badApp, "spread")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to read history") {
		t.Fatalf("unexpected bad spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = runApp(t, app, "gain")
	if code != 0 || !strings.Contains(stdout, "commands:") || stderr != "" {
		t.Fatalf("unexpected gain alias output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}
