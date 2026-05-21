package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/cli"
	"szr/internal/config"
	"szr/internal/history"
	"szr/test/testutil"
)

func TestErrorsAndSpread(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	testutil.WriteExecutable(t, binDir, "rg", "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"__error__\" ]; then\n    echo \"bad rg\" >&2\n    exit 2\n  fi\n  if [ \"$arg\" = \"nomatch\" ]; then\n    exit 1\n  fi\ndone\necho \"file.go:12:match one\"\n")

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
			code, stdout, stderr := testutil.RunApp(t, app, tc.args...)
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

	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || !strings.Contains(stdout, "commands:") || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--history")
	if code != 0 || !strings.Contains(stdout, "recent:") {
		t.Fatalf("unexpected spread history output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil || payload.Commands == 0 {
		t.Fatalf("unexpected spread json payload: %#v err=%v", payload, err)
	}

	emptyApp := testutil.NewTestApp(t)
	code, stdout, stderr = testutil.RunApp(t, emptyApp, "spread")
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
	badApp := cli.NewWithDependencies("test", config.Default(), badPaths, badStore, testutil.AppEngine(t, badPaths))
	code, stdout, stderr = testutil.RunApp(t, badApp, "spread")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to read history") {
		t.Fatalf("unexpected bad spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "gain")
	if code != 0 || !strings.Contains(stdout, "commands:") || stderr != "" {
		t.Fatalf("unexpected gain alias output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}
