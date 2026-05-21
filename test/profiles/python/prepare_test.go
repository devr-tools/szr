package profiles_test

import (
	"reflect"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestPytestProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "pytest")

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

	if got := profile.Prepare(engine.Invocation{Command: []string{"pytest", "tests"}}); !reflect.DeepEqual(got, []string{"pytest", "tests", "-q", "--tb=short", "--color=no", "-ra"}) {
		t.Fatalf("unexpected pytest prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"python", "-m", "pytest", "tests"}}); !reflect.DeepEqual(got, []string{"python", "-m", "pytest", "tests", "-q", "--tb=short", "--color=no", "-ra"}) {
		t.Fatalf("unexpected python -m pytest prepare: %#v", got)
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"uv", "run", "pytest", "tests"}}); !reflect.DeepEqual(got, []string{"uv", "run", "pytest", "tests", "-q", "--tb=short", "--color=no", "-ra"}) {
		t.Fatalf("unexpected uv run pytest prepare: %#v", got)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"pytest", "-vv", "--tb=long", "--color=yes", "-rfE"}})
	if want := []string{"pytest", "-vv", "--tb=long", "--color=yes", "-rfE"}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit pytest flags to be preserved: %#v", preserved)
	}

	partial := profile.Prepare(engine.Invocation{Command: []string{"pytest", "--tb=short"}})
	if want := []string{"pytest", "--tb=short", "-q", "--color=no", "-ra"}; !reflect.DeepEqual(partial, want) {
		t.Fatalf("unexpected partial pytest prepare: %#v", partial)
	}
}
