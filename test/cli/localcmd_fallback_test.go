package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/test/testutil"
)

// The local builtins (ls/find/grep) must only handle argv shapes they fully
// understand. Anything else is delegated through the engine to the native
// binary, preserving the user's arguments and the native exit code while
// profile filtering still applies. Each fake native binary records its argv
// to a side-channel file so the tests can verify what was actually executed.
func TestLocalBuiltinNativeFallbacks(t *testing.T) {
	app := testutil.NewTestApp(t)

	argsDir := t.TempDir()
	grepArgs := filepath.Join(argsDir, "grep.args")
	findArgs := filepath.Join(argsDir, "find.args")
	lsArgs := filepath.Join(argsDir, "ls.args")

	nativeDir := t.TempDir()
	testutil.WriteExecutable(t, nativeDir, "grep", "#!/bin/sh\nprintf '%s\\n' \"$*\" > "+grepArgs+"\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"NOPE\" ]; then\n    exit 1\n  fi\ndone\necho \"src/app.go:3:TODO fix parser\"\n")
	testutil.WriteExecutable(t, nativeDir, "find", "#!/bin/sh\nprintf '%s\\n' \"$*\" > "+findArgs+"\nif [ \"$1\" = \"failroot\" ]; then\n  echo \"find: failroot: no such directory\" >&2\n  exit 4\nfi\necho \"./src/app.go\"\necho \"./src/util.go\"\n")
	testutil.WriteExecutable(t, nativeDir, "ls", "#!/bin/sh\nprintf '%s\\n' \"$*\" > "+lsArgs+"\necho \"total 8\"\necho \"-rw-r--r--  1 dev dev  120 Jan  1 10:00 app.go\"\n")

	rgDir := t.TempDir()
	testutil.WriteExecutable(t, rgDir, "rg", "#!/bin/sh\necho \"file.go:12:match one\"\n")

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	pathWithRg := rgDir + string(os.PathListSeparator) + nativeDir
	pathWithoutRg := nativeDir

	cases := []struct {
		name        string
		path        string
		args        []string
		wantCode    int
		wantOutput  []string
		argsFile    string
		wantArgs    []string
		wantBuiltin bool
	}{
		{
			name:       "grep flag shape delegates even with rg present",
			path:       pathWithRg,
			args:       []string{"grep", "-rn", "TODO", "src"},
			wantCode:   0,
			wantOutput: []string{"app.go", "TODO"},
			argsFile:   grepArgs,
			wantArgs:   []string{"-rn TODO src"},
		},
		{
			name:       "grep falls back to native grep when rg is missing",
			path:       pathWithoutRg,
			args:       []string{"grep", "-rn", "TODO", "src"},
			wantCode:   0,
			wantOutput: []string{"app.go", "TODO"},
			argsFile:   grepArgs,
			wantArgs:   []string{"-rn TODO src"},
		},
		{
			name:     "grep fallback preserves no-match exit code",
			path:     pathWithoutRg,
			args:     []string{"grep", "-rn", "NOPE", "src"},
			wantCode: 1,
			argsFile: grepArgs,
			wantArgs: []string{"-rn NOPE src"},
		},
		{
			name:        "grep builtin still used when rg present",
			path:        pathWithRg,
			args:        []string{"grep", "match", "."},
			wantCode:    0,
			wantOutput:  []string{"match one"},
			argsFile:    grepArgs,
			wantBuiltin: true,
		},
		{
			name:       "find native predicate delegates",
			path:       pathWithRg,
			args:       []string{"find", ".", "-name", "*.go"},
			wantCode:   0,
			wantOutput: []string{"app.go"},
			argsFile:   findArgs,
			wantArgs:   []string{"-name *.go"},
		},
		{
			name:       "find not path predicate delegates",
			path:       pathWithRg,
			args:       []string{"find", ".", "-type", "f", "-not", "-path", "*/.git/*"},
			wantCode:   0,
			wantOutput: []string{"app.go"},
			argsFile:   findArgs,
			wantArgs:   []string{"-type f -not -path */.git/*"},
		},
		{
			name:       "find multiple roots delegate",
			path:       pathWithRg,
			args:       []string{"find", "one", "two"},
			wantCode:   0,
			wantOutput: []string{"app.go"},
			argsFile:   findArgs,
			wantArgs:   []string{"one two"},
		},
		{
			name:       "find delegation preserves exit code",
			path:       pathWithRg,
			args:       []string{"find", "failroot", "-type", "f"},
			wantCode:   4,
			wantOutput: []string{"no such directory"},
			argsFile:   findArgs,
			wantArgs:   []string{"failroot -type f"},
		},
		{
			name:        "find builtin still used for supported flags",
			path:        pathWithRg,
			args:        []string{"find", root, "--name", "*.go"},
			wantCode:    0,
			wantOutput:  []string{"1 matches", "b.go"},
			argsFile:    findArgs,
			wantBuiltin: true,
		},
		{
			name:       "ls with flags delegates",
			path:       pathWithRg,
			args:       []string{"ls", "-la"},
			wantCode:   0,
			wantOutput: []string{"app.go"},
			argsFile:   lsArgs,
			wantArgs:   []string{"-la"},
		},
		{
			name:     "ls with multiple paths delegates",
			path:     pathWithRg,
			args:     []string{"ls", "one", "two"},
			wantCode: 0,
			argsFile: lsArgs,
			wantArgs: []string{"one two"},
		},
		{
			name:        "ls builtin still used for single path",
			path:        pathWithRg,
			args:        []string{"ls", root},
			wantCode:    0,
			wantOutput:  []string{"b.go"},
			argsFile:    lsArgs,
			wantBuiltin: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			if err := os.RemoveAll(tc.argsFile); err != nil {
				t.Fatalf("reset args file: %v", err)
			}

			code, stdout, stderr := testutil.RunApp(t, app, tc.args...)
			combined := stdout + stderr
			if code != tc.wantCode {
				t.Fatalf("unexpected code %d want %d stdout=%q stderr=%q", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantOutput {
				if !strings.Contains(combined, want) {
					t.Fatalf("expected output to contain %q, got stdout=%q stderr=%q", want, stdout, stderr)
				}
			}

			if tc.wantBuiltin {
				if _, err := os.Stat(tc.argsFile); !os.IsNotExist(err) {
					t.Fatalf("expected builtin handling, but native binary was invoked (args file %s exists)", tc.argsFile)
				}
				return
			}

			recorded, err := os.ReadFile(tc.argsFile)
			if err != nil {
				t.Fatalf("expected delegation to native binary, args file missing: %v (stdout=%q stderr=%q)", err, stdout, stderr)
			}
			for _, want := range tc.wantArgs {
				if !strings.Contains(string(recorded), want) {
					t.Fatalf("expected native argv to contain %q, got %q", want, string(recorded))
				}
			}
		})
	}
}
