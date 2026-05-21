package profiles_test

import (
	"reflect"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
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
	want := []string{"rg", "--color=never", "--no-heading", "-H", "-n", "todo", "."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected ripgrep prepare: %#v", got)
	}

	preserved := profile.Prepare(engine.Invocation{Command: []string{"rg", "--color=always", "--heading", "-H", "-n", "todo", "."}})
	if want := []string{"rg", "--color=always", "--heading", "-H", "-n", "todo", "."}; !reflect.DeepEqual(preserved, want) {
		t.Fatalf("expected explicit ripgrep flags to be preserved: %#v", preserved)
	}
}
