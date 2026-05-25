package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/test/testutil"
)

func TestRunRoutes(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\ncase \"$1\" in\nstatus)\n  echo \"## main...origin/main\"\n  echo \"M  README.md\"\n  ;;\nlog)\n  echo \"abc123 first\"\n  echo \"def456 second\"\n  ;;\ndiff)\n  echo \"diff --git a/a.go b/a.go\"\n  echo \" a.go | 2 +-\"\n  echo \" 1 file changed, 1 insertion(+), 1 deletion(-)\"\n  ;;\nesac\n")
	testutil.WriteExecutable(t, binDir, "go", "#!/bin/sh\ncase \"$1\" in\ntest)\n  echo '{\"Action\":\"pass\",\"Package\":\"pkg/pass\"}'\n  echo '{\"Action\":\"fail\",\"Package\":\"pkg/fail\"}'\n  echo '{\"Action\":\"fail\",\"Package\":\"pkg/fail\",\"Test\":\"TestSad\"}'\n  ;;\nbuild)\n  echo 'compile error' >&2\n  exit 1\n  ;;\nvet)\n  echo 'warning: suspicious' >&2\n  exit 1\n  ;;\nesac\n")
	testutil.WriteExecutable(t, binDir, "echoer", "#!/bin/sh\necho plain-output\n")
	testutil.WriteExecutable(t, binDir, "noisy", "#!/bin/sh\necho FAIL one\necho note >&2\n")
	testutil.WriteExecutable(t, binDir, "rg", "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"__error__\" ]; then\n    echo \"bad rg\" >&2\n    exit 2\n  fi\n  if [ \"$arg\" = \"nomatch\" ]; then\n    exit 1\n  fi\ndone\nif [ \"$1\" = \"__missing__\" ]; then\n  exit 1\nfi\necho \"file.go:12:match one\"\necho \"file.go:20:match two\"\n")

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
	if err := os.MkdirAll(filepath.Join(root, "dir", "sub", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}

	cases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
		stdin      string
	}{
		{"help empty", nil, 0, []string{`szr or "sizer" is a token-aware CLI proxy built in Go`, "Setup:"}, nil, ""},
		{"help flag", []string{"--help"}, 0, []string{"Setup:", "Insight:", "Discover:", "szr commands", "--reasoning-budget <standard|agent>", "szr uninstall codex|claude-code|cursor|..."}, nil, ""},
		{"help ultra", []string{"-u", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose long", []string{"--verbose", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose exact", []string{"-vv", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"help verbose counted", []string{"-vvvv", "help"}, 0, []string{"Setup:"}, nil, ""},
		{"commands", []string{"commands"}, 0, []string{"commands", "Execution:", "Local Tools:", "Install:", "szr rg <pattern> [path]", "szr uninstall codex"}, nil, ""},
		{"version", []string{"--version"}, 0, []string{"szr test"}, nil, ""},
		{"profiles", []string{"profiles"}, 0, []string{"git-status", "generic-summary"}, nil, ""},
		{"doctor", []string{"doctor"}, 0, []string{"version: test", "reasoning budget mode: standard", "go:", "git:", "rg:"}, nil, ""},
		{"self doctor", []string{"self", "doctor"}, 0, []string{"version: test", "install target:", "config dir:"}, nil, ""},
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
		{"rg external", []string{"rg", "match", "."}, 0, []string{"file.go (2 matches)"}, nil, ""},
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
				code, stdout, stderr = testutil.RunApp(t, app, tc.args...)
			}
			if tc.stdin != "" {
				testutil.WithStdin(t, tc.stdin, run)
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

func TestRGExternalMissingShowsInstallHint(t *testing.T) {
	app := testutil.NewTestApp(t)
	t.Setenv("PATH", t.TempDir())

	code, stdout, stderr := testutil.RunApp(t, app, "rg", "needle", ".")
	if code != 1 || stdout != "" {
		t.Fatalf("unexpected rg missing stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"szr: `rg` is not installed or not on PATH",
		"szr: install ripgrep to use `szr rg ...`",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestDoctorMarksRipgrepOptional(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\nexit 0\n")
	testutil.WriteExecutable(t, binDir, "go", "#!/bin/sh\nexit 0\n")

	code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "rg: missing (optional; only needed for `szr rg`)") {
		t.Fatalf("expected optional rg status, got %q", stdout)
	}
}
