package engine

import (
	"reflect"
	"testing"
)

func TestUnwrapShellWrapperShapes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "combined -lc with setup source",
			args: []string{"/bin/zsh", "-lc", "source ~/.env && git status"},
			want: []string{"git", "status"},
		},
		{
			name: "separate -l -c flags",
			args: []string{"bash", "-l", "-c", "go test ./..."},
			want: []string{"go", "test", "./..."},
		},
		{
			name: "interactive login cluster -lic",
			args: []string{"zsh", "-lic", "git diff --stat"},
			want: []string{"git", "diff", "--stat"},
		},
		{
			name: "quoted inner words with spaces",
			args: []string{"sh", "-c", `git commit -m "hello world"`},
			want: []string{"git", "commit", "-m", "hello world"},
		},
		{
			name: "single quoted word",
			args: []string{"dash", "-c", "grep 'two words' file.txt"},
			want: []string{"grep", "two words", "file.txt"},
		},
		{
			name: "setup chain with cd and assignments",
			args: []string{"zsh", "-lc", "cd /tmp; FOO=1 BAR=2; go vet ./..."},
			want: []string{"go", "vet", "./..."},
		},
		{
			name: "export and dot-source setup",
			args: []string{"bash", "-c", ". ./env.sh && export PATH=/x:$PATH && node -e 'throw 1'"},
			want: []string{"node", "-e", "throw 1"},
		},
		{
			name: "trailing positional args are ignored for matching",
			args: []string{"sh", "-c", "git status", "sh", "extra"},
			want: []string{"git", "status"},
		},
		{
			name: "backslash escaped space",
			args: []string{"sh", "-c", `cat my\ file.txt`},
			want: []string{"cat", "my file.txt"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrap, inner, ok := unwrapShellWrapper(tc.args)
			if !ok {
				t.Fatalf("expected unwrap of %v", tc.args)
			}
			if !reflect.DeepEqual(inner, tc.want) {
				t.Fatalf("unexpected inner argv: got %v want %v", inner, tc.want)
			}
			if !reflect.DeepEqual(wrap.Original, tc.args) {
				t.Fatalf("expected original argv preserved, got %v", wrap.Original)
			}
		})
	}
}

func TestUnwrapShellWrapperRejectsCompoundCommands(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"pipe", []string{"zsh", "-lc", "git diff | head -20"}},
		{"or chain", []string{"bash", "-c", "make build || make fallback"}},
		{"redirection", []string{"sh", "-c", "go test ./... > out.log"}},
		{"input redirection", []string{"sh", "-c", "psql < dump.sql"}},
		{"command substitution", []string{"bash", "-c", "echo $(git rev-parse HEAD)"}},
		{"backticks", []string{"bash", "-c", "echo `date`"}},
		{"two real commands", []string{"zsh", "-lc", "go build ./... && go test ./..."}},
		{"background job", []string{"sh", "-c", "sleep 5 &"}},
		{"subshell", []string{"sh", "-c", "(cd /tmp && ls)"}},
		{"unterminated quote", []string{"sh", "-c", "echo 'oops"}},
		{"comment only", []string{"sh", "-c", "# nothing"}},
		{"nested wrapper", []string{"zsh", "-lc", "bash -c 'git status'"}},
		{"no -c flag", []string{"zsh", "-l", "script.sh"}},
		{"long option before -c", []string{"bash", "--norc", "-c", "git status"}},
		{"missing command string", []string{"bash", "-c"}},
		{"not a shell", []string{"python3", "-c", "print(1)"}},
		{"empty command string", []string{"sh", "-c", "   "}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := unwrapShellWrapper(tc.args); ok {
				t.Fatalf("expected %v to stay wrapped", tc.args)
			}
		})
	}
}

func TestShellWrapExecCommandTranslatesPrepareRewrites(t *testing.T) {
	args := []string{"/bin/zsh", "-lc", "source /dev/null && go test ./..."}
	wrap, inner, ok := unwrapShellWrapper(args)
	if !ok || !wrap.LiteralSafe {
		t.Fatalf("expected literal-safe unwrap, got wrap=%#v ok=%t", wrap, ok)
	}

	unchanged := wrap.execCommand(inner, []string{"go", "test", "./..."})
	if !reflect.DeepEqual(unchanged, args) {
		t.Fatalf("expected original argv without prepare rewrite, got %v", unchanged)
	}

	rewritten := wrap.execCommand(inner, []string{"go", "test", "-json", "./..."})
	want := []string{"/bin/zsh", "-lc", "source /dev/null && go test -json ./..."}
	if !reflect.DeepEqual(rewritten, want) {
		t.Fatalf("expected rebuilt wrapper argv, got %v", rewritten)
	}
}

func TestShellWrapExecCommandQuotesRewrittenWords(t *testing.T) {
	args := []string{"sh", "-c", "git status"}
	wrap, inner, ok := unwrapShellWrapper(args)
	if !ok {
		t.Fatalf("expected unwrap of %v", args)
	}
	rewritten := wrap.execCommand(inner, []string{"git", "status", "--short", "two words"})
	want := []string{"sh", "-c", "git status --short 'two words'"}
	if !reflect.DeepEqual(rewritten, want) {
		t.Fatalf("expected quoted rebuild, got %v", rewritten)
	}
}

func TestShellWrapExecCommandSuppressesUnsafeRewrites(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"variable expansion", []string{"sh", "-c", "echo $HOME"}},
		{"glob argument", []string{"sh", "-c", "ls *.go"}},
		{"tilde path", []string{"sh", "-c", "cat ~/notes.txt"}},
		{"double quoted expansion", []string{"bash", "-c", `go test "$PKG"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrap, inner, ok := unwrapShellWrapper(tc.args)
			if !ok {
				t.Fatalf("expected unwrap of %v", tc.args)
			}
			if wrap.LiteralSafe {
				t.Fatalf("expected non-literal-safe wrap for %v", tc.args)
			}
			got := wrap.execCommand(inner, append(append([]string(nil), inner...), "--extra"))
			if !reflect.DeepEqual(got, tc.args) {
				t.Fatalf("expected prepare suppression to run original argv, got %v", got)
			}
		})
	}
}

func TestUnwrapShellWrapperKeepsSetupPrefixVerbatim(t *testing.T) {
	wrap, _, ok := unwrapShellWrapper([]string{"zsh", "-lc", "source ~/.env  &&  git status"})
	if !ok {
		t.Fatal("expected unwrap")
	}
	if wrap.Prefix != "source ~/.env  &&  " {
		t.Fatalf("unexpected prefix: %q", wrap.Prefix)
	}
	if wrap.CommandArg != 2 {
		t.Fatalf("unexpected command arg index: %d", wrap.CommandArg)
	}
}
