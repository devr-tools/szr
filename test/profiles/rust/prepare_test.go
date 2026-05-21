package profiles_test

import (
	"reflect"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestCargoProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	cargoTest := testutil.FindProfile(t, list, "cargo-test")
	if !cargoTest.Match(engine.Invocation{Display: []string{"cargo", "test"}}) {
		t.Fatal("expected cargo test to match")
	}
	if !cargoTest.Match(engine.Invocation{Display: []string{"test", "cargo", "test"}, Command: []string{"cargo", "test"}}) {
		t.Fatal("expected wrapped cargo test to match")
	}
	if cargoTest.Match(engine.Invocation{Display: []string{"cargo", "bench"}}) {
		t.Fatal("did not expect cargo bench to match cargo-test profile")
	}
	if got := cargoTest.Prepare(engine.Invocation{Command: []string{"cargo", "test"}}); !reflect.DeepEqual(got, []string{"cargo", "test", "--message-format=short", "--quiet"}) {
		t.Fatalf("unexpected cargo test prepare: %#v", got)
	}
	if got := cargoTest.Prepare(engine.Invocation{Command: []string{"cargo", "test", "--", "--nocapture"}}); !reflect.DeepEqual(got, []string{"cargo", "test", "--message-format=short", "--quiet", "--", "--nocapture"}) {
		t.Fatalf("expected cargo test message format before --, got %#v", got)
	}
	if got := cargoTest.Prepare(engine.Invocation{Command: []string{"cargo", "test", "--message-format=json"}}); !reflect.DeepEqual(got, []string{"cargo", "test", "--message-format=json", "--quiet"}) {
		t.Fatalf("expected explicit cargo message format to be preserved: %#v", got)
	}

	cargoBuild := testutil.FindProfile(t, list, "cargo-build")
	if !cargoBuild.Match(engine.Invocation{Display: []string{"cargo", "build"}}) || !cargoBuild.Match(engine.Invocation{Display: []string{"cargo", "clippy"}}) {
		t.Fatal("expected cargo build profile to match build and clippy")
	}
	if got := cargoBuild.Prepare(engine.Invocation{Command: []string{"cargo", "clippy"}}); !reflect.DeepEqual(got, []string{"cargo", "clippy", "--message-format=short", "--quiet"}) {
		t.Fatalf("unexpected cargo clippy prepare: %#v", got)
	}
}
