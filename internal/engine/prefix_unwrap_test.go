package engine

import (
	"reflect"
	"testing"
)

func TestStripTransparentPrefixShapes(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		prefix []string
		inner  []string
	}{
		{
			name:   "env unset flag",
			args:   []string{"env", "-u", "GOROOT", "go", "test", "./..."},
			prefix: []string{"env", "-u", "GOROOT"},
			inner:  []string{"go", "test", "./..."},
		},
		{
			name:   "env attached unset flag",
			args:   []string{"env", "-uGOROOT", "go", "test"},
			prefix: []string{"env", "-uGOROOT"},
			inner:  []string{"go", "test"},
		},
		{
			name:   "env assignments",
			args:   []string{"env", "FOO=bar", "BAZ=1", "dotnet", "build"},
			prefix: []string{"env", "FOO=bar", "BAZ=1"},
			inner:  []string{"dotnet", "build"},
		},
		{
			name:   "env by absolute path",
			args:   []string{"/usr/bin/env", "FOO=bar", "git", "status"},
			prefix: []string{"/usr/bin/env", "FOO=bar"},
			inner:  []string{"git", "status"},
		},
		{
			name:   "command builtin",
			args:   []string{"command", "dotnet", "build"},
			prefix: []string{"command"},
			inner:  []string{"dotnet", "build"},
		},
		{
			name:   "command -p",
			args:   []string{"command", "-p", "git", "status"},
			prefix: []string{"command", "-p"},
			inner:  []string{"git", "status"},
		},
		{
			name:   "nice with priority",
			args:   []string{"nice", "-n", "10", "go", "test"},
			prefix: []string{"nice", "-n", "10"},
			inner:  []string{"go", "test"},
		},
		{
			name:   "bare time",
			args:   []string{"time", "go", "build", "./..."},
			prefix: []string{"time"},
			inner:  []string{"go", "build", "./..."},
		},
		{
			name:   "leading assignments",
			args:   []string{"FOO=1", "BAR=2", "go", "vet"},
			prefix: []string{"FOO=1", "BAR=2"},
			inner:  []string{"go", "vet"},
		},
		{
			name:   "stacked wrappers",
			args:   []string{"env", "FOO=1", "nice", "go", "test"},
			prefix: []string{"env", "FOO=1", "nice"},
			inner:  []string{"go", "test"},
		},
		{
			name:   "stack stops at command-only wrapper",
			args:   []string{"nice", "env", "FOO=1"},
			prefix: []string{"nice"},
			inner:  []string{"env", "FOO=1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix, inner, ok := stripTransparentPrefix(tc.args)
			if !ok {
				t.Fatalf("expected strip of %v", tc.args)
			}
			if !reflect.DeepEqual(prefix, tc.prefix) || !reflect.DeepEqual(inner, tc.inner) {
				t.Fatalf("unexpected strip: got prefix=%v inner=%v want prefix=%v inner=%v", prefix, inner, tc.prefix, tc.inner)
			}
		})
	}
}

func TestStripTransparentPrefixRejectsNonPrefixForms(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"bare env", []string{"env"}},
		{"env assignments only", []string{"env", "FOO=bar"}},
		{"env unset without command", []string{"env", "-u", "GOROOT"}},
		{"env dangling -u", []string{"env", "-u"}},
		{"env unmodeled flag", []string{"env", "-i", "go", "test"}},
		{"command without inner", []string{"command"}},
		{"command -v query", []string{"command", "-v", "git"}},
		{"nice dangling -n", []string{"nice", "-n"}},
		{"nice unmodeled flag", []string{"nice", "--adjustment=5", "go", "test"}},
		{"time with flag", []string{"time", "-v", "go", "test"}},
		{"assignments only", []string{"FOO=1", "BAR=2"}},
		{"plain command", []string{"git", "status"}},
		{"empty argv", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := stripTransparentPrefix(tc.args); ok {
				t.Fatalf("expected %v to stay unstripped", tc.args)
			}
		})
	}
}

func TestUnwrapShellInvocationStripsPrefixes(t *testing.T) {
	args := []string{"env", "-u", "GOROOT", "go", "test", "./..."}
	inv := unwrapShellInvocation(Invocation{Command: args, Display: args})
	if !reflect.DeepEqual(inv.Command, []string{"go", "test", "./..."}) {
		t.Fatalf("unexpected matching command: %v", inv.Command)
	}
	if !reflect.DeepEqual(inv.Display, []string{"go", "test", "./..."}) {
		t.Fatalf("unexpected display command: %v", inv.Display)
	}
	if inv.ShellWrap == nil || inv.ShellWrap.CommandArg != -1 {
		t.Fatalf("expected argv-prefix wrap, got %#v", inv.ShellWrap)
	}
	if !reflect.DeepEqual(inv.ShellWrap.Original, args) {
		t.Fatalf("expected original argv preserved, got %v", inv.ShellWrap.Original)
	}
}

func TestUnwrapShellInvocationStripsPrefixInsideWrapper(t *testing.T) {
	args := []string{"bash", "-c", "env FOO=1 dotnet build"}
	inv := unwrapShellInvocation(Invocation{Command: args, Display: args})
	if !reflect.DeepEqual(inv.Command, []string{"dotnet", "build"}) {
		t.Fatalf("unexpected matching command: %v", inv.Command)
	}
	if inv.ShellWrap == nil || inv.ShellWrap.CommandArg != 2 {
		t.Fatalf("expected shell wrap preserved, got %#v", inv.ShellWrap)
	}
	if !reflect.DeepEqual(inv.ShellWrap.PrefixWords, []string{"env", "FOO=1"}) {
		t.Fatalf("unexpected prefix words: %v", inv.ShellWrap.PrefixWords)
	}
}

func TestArgvPrefixWrapExecCommand(t *testing.T) {
	args := []string{"env", "-u", "GOROOT", "go", "test", "./..."}
	inv := unwrapShellInvocation(Invocation{Command: args, Display: args})

	unchanged := inv.ShellWrap.execCommand(inv.Command, []string{"go", "test", "./..."})
	if !reflect.DeepEqual(unchanged, args) {
		t.Fatalf("expected original argv without prepare rewrite, got %v", unchanged)
	}

	rewritten := inv.ShellWrap.execCommand(inv.Command, []string{"go", "test", "./...", "-json"})
	want := []string{"env", "-u", "GOROOT", "go", "test", "./...", "-json"}
	if !reflect.DeepEqual(rewritten, want) {
		t.Fatalf("expected spliced prepare rewrite, got %v", rewritten)
	}
}

func TestWrapperPrefixExecCommandRebuildsCommandString(t *testing.T) {
	args := []string{"bash", "-c", "env FOO=1 dotnet build"}
	inv := unwrapShellInvocation(Invocation{Command: args, Display: args})

	unchanged := inv.ShellWrap.execCommand(inv.Command, []string{"dotnet", "build"})
	if !reflect.DeepEqual(unchanged, args) {
		t.Fatalf("expected original argv without prepare rewrite, got %v", unchanged)
	}

	rewritten := inv.ShellWrap.execCommand(inv.Command, []string{"dotnet", "build", "--nologo"})
	want := []string{"bash", "-c", "env FOO=1 dotnet build --nologo"}
	if !reflect.DeepEqual(rewritten, want) {
		t.Fatalf("expected rebuilt wrapper argv with prefix words, got %v", rewritten)
	}
}
