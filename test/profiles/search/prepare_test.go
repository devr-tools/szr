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
	filesProfile := testutil.FindProfile(t, list, "ripgrep-files")
	filesWithMatchesProfile := testutil.FindProfile(t, list, "ripgrep-files-with-matches")

	if !profile.Match(engine.Invocation{Display: []string{"rg", "todo", "."}}) {
		t.Fatal("expected rg to match ripgrep profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"rg", "--json", "todo"}}) {
		t.Fatal("did not expect rg --json to match ripgrep profile")
	}
	if profile.Match(engine.Invocation{Display: []string{"rg", "--files"}}) {
		t.Fatal("did not expect rg --files to match ripgrep profile")
	}
	if !filesProfile.Match(engine.Invocation{Display: []string{"rg", "--files"}}) {
		t.Fatal("expected rg --files to match ripgrep-files profile")
	}
	if filesProfile.Match(engine.Invocation{Display: []string{"rg", "todo", "."}}) {
		t.Fatal("did not expect plain rg search to match ripgrep-files profile")
	}
	if !filesWithMatchesProfile.Match(engine.Invocation{Display: []string{"rg", "--files-with-matches", "todo"}}) {
		t.Fatal("expected rg --files-with-matches to match ripgrep-files-with-matches profile")
	}
	if !filesWithMatchesProfile.Match(engine.Invocation{Display: []string{"rg", "-l", "todo"}}) {
		t.Fatal("expected rg -l to match ripgrep-files-with-matches profile")
	}
	if filesWithMatchesProfile.Match(engine.Invocation{Display: []string{"rg", "--files"}}) {
		t.Fatal("did not expect rg --files to match ripgrep-files-with-matches profile")
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
		"-g", "!" + ".gradle" + "/**",
		"-g", "!" + ".mypy_cache" + "/**",
		"-g", "!" + ".nox" + "/**",
		"-g", "!" + ".nuxt" + "/**",
		"-g", "!" + ".output" + "/**",
		"-g", "!" + ".parcel-cache" + "/**",
		"-g", "!" + ".pnpm-store" + "/**",
		"-g", "!" + ".ruff_cache" + "/**",
		"-g", "!" + ".svelte-kit" + "/**",
		"-g", "!" + ".venv" + "/**",
		"-g", "!" + ".yarn" + "/**",
		"-g", "!" + "out" + "/**",
		"-g", "!" + "tmp" + "/**",
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
		"-g", "!" + ".gradle" + "/**",
		"-g", "!" + ".mypy_cache" + "/**",
		"-g", "!" + ".nox" + "/**",
		"-g", "!" + ".nuxt" + "/**",
		"-g", "!" + ".output" + "/**",
		"-g", "!" + ".parcel-cache" + "/**",
		"-g", "!" + ".pnpm-store" + "/**",
		"-g", "!" + ".ruff_cache" + "/**",
		"-g", "!" + ".svelte-kit" + "/**",
		"-g", "!" + ".venv" + "/**",
		"-g", "!" + ".yarn" + "/**",
		"-g", "!" + "out" + "/**",
		"-g", "!" + "tmp" + "/**",
		"--color=always", "--heading", "-H", "-n", "todo", ".",
	}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit ripgrep flags to be preserved: %#v", preserved)
	}

	scoped := profile.Prepare(engine.Invocation{Command: []string{"rg", "todo", "vendor"}})
	if reflect.DeepEqual(scoped, want) || len(scoped) == len(want) {
		t.Fatalf("did not expect default ripgrep excludes for scoped search: %#v", scoped)
	}

	filesPrepared := filesProfile.Prepare(engine.Invocation{Command: []string{"rg", "--files"}})
	filesWant := []string{
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
		"-g", "!" + ".gradle" + "/**",
		"-g", "!" + ".mypy_cache" + "/**",
		"-g", "!" + ".nox" + "/**",
		"-g", "!" + ".nuxt" + "/**",
		"-g", "!" + ".output" + "/**",
		"-g", "!" + ".parcel-cache" + "/**",
		"-g", "!" + ".pnpm-store" + "/**",
		"-g", "!" + ".ruff_cache" + "/**",
		"-g", "!" + ".svelte-kit" + "/**",
		"-g", "!" + ".venv" + "/**",
		"-g", "!" + ".yarn" + "/**",
		"-g", "!" + "out" + "/**",
		"-g", "!" + "tmp" + "/**",
		"--files",
	}
	if !reflect.DeepEqual(filesPrepared, filesWant) {
		t.Fatalf("unexpected ripgrep-files prepare: %#v", filesPrepared)
	}

	filesWithMatchesPrepared := filesWithMatchesProfile.Prepare(engine.Invocation{Command: []string{"rg", "--files-with-matches", "todo"}})
	filesWithMatchesWant := append(filesWant[:len(filesWant)-1], "--files-with-matches", "todo")
	if !reflect.DeepEqual(filesWithMatchesPrepared, filesWithMatchesWant) {
		t.Fatalf("unexpected ripgrep-files-with-matches prepare: %#v", filesWithMatchesPrepared)
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
	for _, want := range []string{"*/.venv/*", "*/.gradle/*", "*/tmp/*"} {
		found := false
		for _, arg := range prepared {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected find prepare to include %q, got %#v", want, prepared)
		}
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"find", "vendor", "-name", "*.py"}})
	if !reflect.DeepEqual(preserved, []string{"find", "vendor", "-name", "*.py"}) {
		t.Fatalf("expected scoped find command to be preserved, got %#v", preserved)
	}
}
