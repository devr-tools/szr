package engine_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
)

func TestTransparentPrefixRoutesToInnerCommandProfiles(t *testing.T) {
	t.Parallel()
	e := newBuiltinEngine(t)

	cases := []struct {
		name    string
		args    []string
		profile string
	}{
		{"env unset flag before go build", []string{"env", "-u", "GOROOT", "go", "build", "./..."}, "go-build"},
		{"env assignment before dotnet build", []string{"env", "FOO=bar", "dotnet", "build"}, "dotnet-build"},
		{"env assignment before dotnet test", []string{"env", "FOO=bar", "dotnet", "test"}, "dotnet-test"},
		{"absolute env path", []string{"/usr/bin/env", "FOO=bar", "git", "status"}, "git-status"},
		{"nice with priority", []string{"nice", "-n", "10", "go", "test", "./..."}, "go-test-json"},
		{"bare nice", []string{"nice", "go", "test", "./..."}, "go-test-json"},
		{"command builtin", []string{"command", "dotnet", "build"}, "dotnet-build"},
		{"command -p", []string{"command", "-p", "git", "status"}, "git-status"},
		{"bare time prefix", []string{"time", "go", "build", "./..."}, "go-build"},
		{"stacked env and nice", []string{"env", "FOO=1", "nice", "go", "test", "./..."}, "go-test-json"},
		{"leading assignment", []string{"FOO=bar", "dotnet", "build"}, "dotnet-build"},
		{"prefix inside shell wrapper", []string{"sh", "-c", "env FOO=1 git status"}, "git-status"},
		{"assignment inside shell wrapper", []string{"bash", "-c", "FOO=1 dotnet build"}, "dotnet-build"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := e.Explain(engine.Invocation{Command: tc.args, Display: tc.args})
			if profile.Name != tc.profile {
				t.Fatalf("expected %v to route to %q, got %q", tc.args, tc.profile, profile.Name)
			}
		})
	}
}

// A wrapper that consumes the whole argv is the command itself, not a
// transparent prefix: bare `env` and assignments-only forms stay wrapped so
// dedicated profiles can still claim them.
func TestTransparentPrefixKeepsCommandOnlyWrappers(t *testing.T) {
	t.Parallel()
	e := newBuiltinEngine(t)

	cases := []struct {
		name string
		args []string
	}{
		{"bare env", []string{"env"}},
		{"env with assignments only", []string{"env", "FOO=bar"}},
		{"env unset flag without command", []string{"env", "-u", "GOROOT"}},
		{"env with unmodeled flag", []string{"env", "-i", "go", "test"}},
		{"command -v query", []string{"command", "-v", "git"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prepared, _ := e.ExplainPreferences(engine.Invocation{Command: tc.args, Display: tc.args})
			if !reflect.DeepEqual(prepared.Command, tc.args) {
				t.Fatalf("expected %v to stay unwrapped, got matching command %v", tc.args, prepared.Command)
			}
		})
	}
}

// Prefix stripping only changes the matching view: the prepared invocation
// records the original argv as the execution baseline.
func TestTransparentPrefixPreservesOriginalForExecution(t *testing.T) {
	t.Parallel()
	e := newBuiltinEngine(t)

	args := []string{"env", "FOO=bar", "dotnet", "build"}
	prepared, _ := e.ExplainPreferences(engine.Invocation{Command: args, Display: args})
	if !reflect.DeepEqual(prepared.Command, []string{"dotnet", "build"}) {
		t.Fatalf("expected stripped matching command, got %v", prepared.Command)
	}
	if prepared.ShellWrap == nil || !reflect.DeepEqual(prepared.ShellWrap.Original, args) {
		t.Fatalf("expected original argv preserved for execution, got %#v", prepared.ShellWrap)
	}
}

// A profile Prepare rewrite on a prefix-stripped command splices back after
// the verbatim prefix words, so the wrapper still applies at execution.
func TestTransparentPrefixTranslatesPrepareIntoWrapper(t *testing.T) {
	t.Parallel()
	e := newPrepareTranslationEngine(t)
	args := []string{"env", "GREETING=1", "echo", "hello"}
	result, err := e.Execute(context.Background(), engine.Invocation{Command: args, Display: args, Cwd: t.TempDir()}, false)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.ProfileName != "echo-append" {
		t.Fatalf("expected prefix strip to match inner echo profile, got %q", result.ProfileName)
	}
	if !strings.Contains(result.RawCombined, "hello appended") {
		t.Fatalf("expected spliced prepare rewrite in output, got %q", result.RawCombined)
	}
	if result.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", result.ExitCode)
	}
}
