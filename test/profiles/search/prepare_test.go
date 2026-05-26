package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSearchProfilesCapabilities(t *testing.T) {
	list := profiles.Builtins(6)
	for _, name := range []string{"grep", "ripgrep", "ripgrep-files", "ripgrep-files-with-matches", "path-find"} {
		profile := testutil.FindProfile(t, list, name)
		if profile.Capabilities.StructuredMode != engine.StructuredModePreferred {
			t.Fatalf("expected %s to prefer structured mode, got %q", name, profile.Capabilities.StructuredMode)
		}
		if !profile.Capabilities.InjectsPrepareArgs {
			t.Fatalf("expected %s to declare prepare arg injection", name)
		}
		if profile.Capabilities.FastPathBypass != engine.FastPathBypassSafeOnly {
			t.Fatalf("expected %s to use safe-only fast-path bypass, got %q", name, profile.Capabilities.FastPathBypass)
		}
		if !profile.Capabilities.RequireFullCapture {
			t.Fatalf("expected %s to require full capture for recovery", name)
		}
	}
}

func TestRipgrepProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	grepProfile := testutil.FindProfile(t, list, "grep")
	profile := testutil.FindProfile(t, list, "ripgrep")
	filesProfile := testutil.FindProfile(t, list, "ripgrep-files")
	filesWithMatchesProfile := testutil.FindProfile(t, list, "ripgrep-files-with-matches")

	assertProfileMatches(t, grepProfile, []string{"grep", "-rn", "todo", "."}, true)
	assertProfileMatches(t, grepProfile, []string{"grep", "-n", "todo", "."}, false)
	assertProfileMatches(t, grepProfile, []string{"grep", "-rnh", "todo", "."}, false)
	assertProfileMatches(t, profile, []string{"rg", "todo", "."}, true)
	assertProfileMatches(t, profile, []string{"rg", "--json", "todo"}, false)
	assertProfileMatches(t, profile, []string{"rg", "--files"}, false)
	assertProfileMatches(t, filesProfile, []string{"rg", "--files"}, true)
	assertProfileMatches(t, filesProfile, []string{"rg", "todo", "."}, false)
	assertProfileMatches(t, filesWithMatchesProfile, []string{"rg", "--files-with-matches", "todo"}, true)
	assertProfileMatches(t, filesWithMatchesProfile, []string{"rg", "-l", "todo"}, true)
	assertProfileMatches(t, filesWithMatchesProfile, []string{"rg", "--files"}, false)

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

	grepPrepared := grepProfile.Prepare(engine.Invocation{Command: []string{"grep", "-rn", "todo", "."}})
	grepWant := []string{
		"grep",
		"--exclude-dir=.git",
		"--exclude-dir=.next",
		"--exclude-dir=.turbo",
		"--exclude-dir=.cache",
		"--exclude-dir=__pycache__",
		"--exclude-dir=build",
		"--exclude-dir=coverage",
		"--exclude-dir=dist",
		"--exclude-dir=node_modules",
		"--exclude-dir=target",
		"--exclude-dir=vendor",
		"--exclude-dir=.gradle",
		"--exclude-dir=.mypy_cache",
		"--exclude-dir=.nox",
		"--exclude-dir=.nuxt",
		"--exclude-dir=.output",
		"--exclude-dir=.parcel-cache",
		"--exclude-dir=.pnpm-store",
		"--exclude-dir=.ruff_cache",
		"--exclude-dir=.svelte-kit",
		"--exclude-dir=.venv",
		"--exclude-dir=.yarn",
		"--exclude-dir=out",
		"--exclude-dir=tmp",
		"--color=never", "-H", "-rn", "todo", ".",
	}
	if !reflect.DeepEqual(grepPrepared, grepWant) {
		t.Fatalf("unexpected grep prepare: %#v", grepPrepared)
	}

	grepScoped := grepProfile.Prepare(engine.Invocation{Command: []string{"grep", "-rn", "todo", "service/src/backend"}})
	grepScopedWant := []string{"grep", "--color=never", "-H", "-rn", "todo", "service/src/backend"}
	if !reflect.DeepEqual(grepScoped, grepScopedWant) {
		t.Fatalf("unexpected scoped grep prepare: %#v", grepScoped)
	}
}

func assertProfileMatches(t *testing.T, profile engine.Profile, display []string, want bool) {
	t.Helper()
	inv := engine.Classify(engine.Invocation{Display: display})
	if profile.Match(inv) != want {
		t.Fatalf("unexpected match result for %#v", display)
	}
}

func TestFindProfileMatch(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "path-find")

	if !profile.Match(engine.Classify(engine.Invocation{Display: []string{"find", ".", "-name", "*.py"}})) {
		t.Fatal("expected plain find to match path-find profile")
	}
	if profile.Match(engine.Classify(engine.Invocation{Display: []string{"find", ".", "-exec", "rm", "{}", ";"}})) {
		t.Fatal("did not expect destructive find to match path-find profile")
	}
	if profile.Match(engine.Classify(engine.Invocation{Display: []string{"find", ".", "-prune"}})) {
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
