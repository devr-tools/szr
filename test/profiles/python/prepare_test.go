package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPytestProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "pytest")
	advanced := config.Default().Advanced

	if !profile.Match(engine.Invocation{Display: []string{"pytest", "-k", "math"}}) {
		t.Fatal("expected direct pytest to match")
	}
	if !profile.Match(engine.Invocation{Display: []string{"python", "-m", "pytest", "tests"}}) {
		t.Fatal("expected python -m pytest to match")
	}
	if !profile.Match(engine.Invocation{Display: []string{"uv", "run", "pytest", "tests"}}) {
		t.Fatal("expected uv run pytest to match")
	}
	if profile.Match(engine.Invocation{Display: []string{"python", "-m", "unittest"}}) {
		t.Fatal("did not expect unittest to match pytest profile")
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"pytest", "tests"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"pytest", "tests", "-q", "--no-header", "--tb=short", "--color=no", "--disable-warnings", "-ra"}) {
		t.Fatalf("unexpected pytest prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"python", "-m", "pytest", "tests"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"python", "-m", "pytest", "tests", "-q", "--no-header", "--tb=short", "--color=no", "--disable-warnings", "-ra"}) {
		t.Fatalf("unexpected python -m pytest prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"uv", "run", "pytest", "tests"}, Advanced: advanced}); !reflect.DeepEqual(got, []string{"uv", "run", "pytest", "tests", "-q", "--no-header", "--tb=short", "--color=no", "--disable-warnings", "-ra"}) {
		t.Fatalf("unexpected uv run pytest prepare: %#v", got)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"pytest", "-vv", "--tb=long", "--color=yes", "-rfE"}, Advanced: advanced})
	if want := []string{"pytest", "-vv", "--tb=long", "--color=yes", "-rfE", "--no-header", "--disable-warnings"}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit pytest flags to be preserved: %#v", preserved)
	}

	partial := profile.Prepare(engine.Invocation{Command: []string{"pytest", "--tb=short"}, Advanced: advanced})
	if want := []string{"pytest", "--tb=short", "-q", "--no-header", "--color=no", "--disable-warnings", "-ra"}; !reflect.DeepEqual(partial, want) {
		t.Fatalf("unexpected partial pytest prepare: %#v", partial)
	}
}

func TestPythonToolingProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "python-tooling")

	for _, display := range [][]string{
		{"uv", "sync"},
		{"poetry", "install"},
		{"pip", "install", "foo"},
		{"pip3", "install", "foo"},
		{"ruff", "check", "."},
		{"mypy", "src"},
		{"python", "-m", "pip", "install", "foo"},
		{"python", "-m", "ruff", "check", "."},
		{"python", "-m", "mypy", "src"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match python-tooling", display)
		}
	}
	if profile.Match(engine.Invocation{Display: []string{"uv", "run", "pytest"}}) {
		t.Fatal("did not expect uv run pytest to match python-tooling")
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"ruff", "check", "."}}); !reflect.DeepEqual(got, []string{"ruff", "check", ".", "--output-format", "concise"}) {
		t.Fatalf("unexpected ruff prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"mypy", "src"}}); !reflect.DeepEqual(got, []string{"mypy", "src", "--show-error-codes", "--hide-error-context", "--no-color-output"}) {
		t.Fatalf("unexpected mypy prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"python", "-m", "ruff", "check", "."}}); !reflect.DeepEqual(got, []string{"python", "-m", "ruff", "check", ".", "--output-format", "concise"}) {
		t.Fatalf("unexpected python -m ruff prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"poetry", "install"}}); !reflect.DeepEqual(got, []string{"poetry", "install"}) {
		t.Fatalf("expected poetry passthrough, got %#v", got)
	}
}
