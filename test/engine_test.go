package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/internal/rules"
)

func TestEngineHelpers(t *testing.T) {
	cases := []struct {
		stdout string
		stderr string
		want   string
	}{
		{"", "", ""},
		{"out", "", "out"},
		{"", "err", "err"},
		{"out", "err", "out\nerr"},
	}
	for _, tc := range cases {
		if got := engine.CombineStreams(tc.stdout, tc.stderr); got != tc.want {
			t.Fatalf("combine streams mismatch: got %q want %q", got, tc.want)
		}
	}

	if got := engine.SanitizeFileName("***"); got != "output" {
		t.Fatalf("unexpected empty sanitize fallback: %q", got)
	}
	if got := engine.SanitizeFileName("abc-123"); got != "abc_123" {
		t.Fatalf("unexpected sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName("AbC9"); got != "AbC9" {
		t.Fatalf("unexpected uppercase sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName(strings.Repeat("x", 60)); len(got) != 48 {
		t.Fatalf("expected truncated sanitize result, got %d chars", len(got))
	}
}

func TestEngineExecuteAndHistory(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "succeed", "#!/bin/sh\necho stdout\n")
	writeExecutable(t, binDir, "failcmd", "#!/bin/sh\necho stderr >&2\nexit 3\n")
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
	store := history.New(paths.HistoryFile)
	profile := engine.Profile{
		Name: "custom",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "succeed"
		},
		Prepare: func(inv engine.Invocation) []string {
			return inv.Command
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return "rendered"
		},
	}
	blankProfile := engine.Profile{
		Name: "blank",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "blankout"
		},
		Render: func(engine.Invocation, engine.Execution) string {
			return ""
		},
	}
	writeExecutable(t, binDir, "blankout", "#!/bin/sh\necho raw-only\n")

	e := engine.New(cfg, paths, store, []engine.Profile{profile, blankProfile})
	if len(e.Profiles()) != 2 {
		t.Fatalf("unexpected profiles copy length")
	}
	explained := e.Explain(engine.Invocation{Display: []string{"other"}})
	if explained.Name != "passthrough" {
		t.Fatalf("unexpected fallback profile: %#v", explained)
	}

	if _, err := e.Execute(context.Background(), engine.Invocation{}, false); err == nil {
		t.Fatal("expected missing command error")
	}

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"succeed"},
		Display: []string{"succeed"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "rendered" || result.ExitCode != 0 || result.ProfileName != "custom" {
		t.Fatalf("unexpected success result: %#v err=%v", result, err)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"blankout"},
		Display: []string{"blankout"},
		Cwd:     root,
	}, false)
	if err != nil || result.Display != "raw-only" {
		t.Fatalf("expected raw fallback, got %#v err=%v", result, err)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"succeed"},
		Display: []string{"succeed"},
		Cwd:     root,
	}, true)
	if err != nil || result.Display != "stdout" {
		t.Fatalf("expected passthrough result, got %#v err=%v", result, err)
	}

	failResult, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"failcmd"},
		Display: []string{"failcmd", strings.Repeat("x", 60)},
		Cwd:     root,
	}, false)
	if err != nil || failResult.ExitCode != 3 || failResult.TeePath == "" || !strings.Contains(failResult.Display, "[full output:") {
		t.Fatalf("unexpected failing result: %#v err=%v", failResult, err)
	}
	if _, statErr := os.Stat(failResult.TeePath); statErr != nil {
		t.Fatalf("expected tee file: %v", statErr)
	}

	cfgNoTee := cfg
	cfgNoTee.TeeOnFailure = false
	eNoTee := engine.New(cfgNoTee, paths, history.New(filepath.Join(root, "data", "other.jsonl")), nil)
	noTeeResult, err := eNoTee.Execute(context.Background(), engine.Invocation{
		Command: []string{"failcmd"},
		Display: []string{"failcmd"},
		Cwd:     root,
	}, false)
	if err != nil || noTeeResult.TeePath != "" {
		t.Fatalf("expected no tee result: %#v err=%v", noTeeResult, err)
	}

	_, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"does-not-exist"},
		Display: []string{"does-not-exist"},
		Cwd:     root,
	}, false)
	if err == nil {
		t.Fatal("expected exec error")
	}
}

func TestEngineProjectRules(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "argvdump", `#!/bin/sh
printf '%s\n' "$@"
`)
	writeExecutable(t, binDir, "noisy", `#!/bin/sh
echo "FAIL first"
echo "plain second"
echo "plain third"
`)
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
				Name:        "argvdump-local",
				Description: "Project-local arg rewriter",
				Explain:     []string{"Appends --json locally."},
				Match: rules.Match{
					CommandPrefix: []string{"argvdump"},
				},
				Rewrite: rules.Rewrite{
					Mode:         "append",
					Args:         []string{"--json"},
					SkipIfHasAny: []string{"--json"},
				},
				Render: rules.Render{
					Mode: "passthrough",
				},
			},
			{
				Name: "display-wrapper",
				Match: rules.Match{
					DisplayPrefix: []string{"summary", "npm", "test"},
				},
			},
			{
				Name: "noisy-local",
				Match: rules.Match{
					CommandPrefix: []string{"noisy"},
				},
				Render: rules.Render{
					Mode:     "failure",
					MaxLines: 2,
				},
			},
			{
				Name: "git-status",
				Match: rules.Match{
					CommandPrefix: []string{"git", "status"},
				},
				Render: rules.Render{
					Mode:     "compact",
					MaxLines: 1,
				},
			},
		},
	}

	store := history.New(paths.HistoryFile)
	builtins := []engine.Profile{
		{
			Name: "git-status",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) >= 2 && inv.Command[0] == "git" && inv.Command[1] == "status"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return "builtin"
			},
		},
		{
			Name: "fallback",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Command) > 0 && inv.Command[0] == "fallback"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return "fallback"
			},
		},
	}
	e := engine.New(cfg, paths, store, builtins)

	profiles := e.Profiles()
	if len(profiles) != 5 {
		t.Fatalf("unexpected merged profile count: %d", len(profiles))
	}
	gitStatusCount := 0
	for _, profile := range profiles {
		if profile.Name == "git-status" {
			gitStatusCount++
		}
	}
	if gitStatusCount != 1 {
		t.Fatalf("expected custom git-status to dedupe built-in, got %d", gitStatusCount)
	}

	explained := e.Explain(engine.Invocation{
		Command: []string{"argvdump", "target"},
		Display: []string{"argvdump", "target"},
	})
	if explained.Name != "argvdump-local" || len(explained.Explain) == 0 {
		t.Fatalf("unexpected explain result: %#v", explained)
	}

	explained = e.Explain(engine.Invocation{
		Command: []string{"npm", "test"},
		Display: []string{"summary", "npm", "test"},
	})
	if explained.Name != "display-wrapper" {
		t.Fatalf("expected display-prefix match, got %#v", explained)
	}

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "target"},
		Display: []string{"argvdump", "target"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute argvdump with rewrite: %v", err)
	}
	if got := strings.Split(result.Display, "\n"); len(got) != 2 || got[0] != "target" || got[1] != "--json" {
		t.Fatalf("unexpected rewritten output: %q", result.Display)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"argvdump", "target", "--json"},
		Display: []string{"argvdump", "target", "--json"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute argvdump with skip flag: %v", err)
	}
	if strings.Count(result.Display, "--json") != 1 {
		t.Fatalf("expected rewrite skip guard, got %q", result.Display)
	}

	result, err = e.Execute(context.Background(), engine.Invocation{
		Command: []string{"noisy"},
		Display: []string{"noisy"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute noisy render: %v", err)
	}
	if !strings.Contains(result.Display, "FAIL first") {
		t.Fatalf("unexpected failure render output: %q", result.Display)
	}
	if strings.Contains(result.Display, "plain second") || strings.Contains(result.Display, "plain third") {
		t.Fatalf("expected failure-focused render, got %q", result.Display)
	}

	explained = e.Explain(engine.Invocation{
		Command: []string{"git", "status"},
		Display: []string{"git", "status"},
	})
	if explained.Name != "git-status" || explained.Render == nil {
		t.Fatalf("expected custom git-status override, got %#v", explained)
	}
	if got := explained.Render(engine.Invocation{}, engine.Execution{Stdout: "one\ntwo\nthree\n"}); got != "one\n... +2 more lines" {
		t.Fatalf("unexpected compact render output: %q", got)
	}
}
