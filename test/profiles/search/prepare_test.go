package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRipgrepProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "ripgrep")

	if !profile.Match(engine.Invocation{Display: []string{"rg", "todo", "."}}) {
		t.Fatal("expected rg to match ripgrep profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"rg", "--json", "todo"}}) {
		t.Fatal("did not expect rg --json to match ripgrep profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"rg", "--files"}}) {
		t.Fatal("did not expect rg --files to match ripgrep profile")
	}

	got := profile.Prepare(engine.Invocation{Command: []string{"rg", "todo", "."}})
	want := []string{
		"rg",
		"-g", "!" + ".git" + "/**",
		"-g", "!" + ".next" + "/**",
		"-g", "!" + ".turbo" + "/**",
		"-g", "!" + ".cache" + "/**",
		"-g", "!" + "__pycache__" + "/**",
		"-g", "!" + "build" + "/**",
		"-g", "!" + "coverage" + "/**",
		"-g", "!" + "dist" + "/**",
		"-g", "!" + "node_modules" + "/**",
		"-g", "!" + "target" + "/**",
		"-g", "!" + "vendor" + "/**",
		"--color=never", "--no-heading", "-H", "-n", "todo", ".",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ripgrep prepare: %#v", got)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"rg", "--color=always", "--heading", "-H", "-n", "todo", "."}})
	if want := []string{
		"rg",
		"-g", "!" + ".git" + "/**",
		"-g", "!" + ".next" + "/**",
		"-g", "!" + ".turbo" + "/**",
		"-g", "!" + ".cache" + "/**",
		"-g", "!" + "__pycache__" + "/**",
		"-g", "!" + "build" + "/**",
		"-g", "!" + "coverage" + "/**",
		"-g", "!" + "dist" + "/**",
		"-g", "!" + "node_modules" + "/**",
		"-g", "!" + "target" + "/**",
		"-g", "!" + "vendor" + "/**",
		"--color=always", "--heading", "-H", "-n", "todo", ".",
	}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit ripgrep flags to be preserved: %#v", preserved)
	}

	scoped := profile.Prepare(engine.Invocation{Command: []string{"rg", "todo", "vendor"}})
	if reflect.DeepEqual(scoped, want) || len(scoped) == len(want) {
		t.Fatalf("did not expect default ripgrep excludes for scoped search: %#v", scoped)
	}
}

func TestFindProfileMatch(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "path-find")

	if !profile.Match(engine.Invocation{Display: []string{"find", ".", "-name", "*.py"}}) {
		t.Fatal("expected plain find to match path-find profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"find", ".", "-exec", "rm", "{}", ";"}}) {
		t.Fatal("did not expect destructive find to match path-find profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"find", ".", "-prune"}}) {
		t.Fatal("did not expect prune-heavy find to match path-find profile")
	}

	prepared := profile.Prepare(engine.Invocation{Command: []string{"find", ".", "-name", "*.py"}})
	if len(prepared) <= 4 {
		t.Fatalf("expected default find excludes to be injected, got %#v", prepared)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"find", "vendor", "-name", "*.py"}})
	if !reflect.DeepEqual(preserved, []string{"find", "vendor", "-name", "*.py"}) {
		t.Fatalf("expected scoped find command to be preserved, got %#v", preserved)
	}
}
