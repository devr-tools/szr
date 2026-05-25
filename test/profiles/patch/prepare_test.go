package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestPatchDiffProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "patch-diff")

	for _, display := range [][]string{
		{"diff", "-u", "a", "b"},
		{"patch", "-p1", "<", "fix.patch"},
		{"git", "apply", "fix.patch"},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match patch-diff", display)
		}
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"git", "apply", "fix.patch"}}); !reflect.DeepEqual(got, []string{"git", "apply", "fix.patch"}) {
		t.Fatalf("expected patch-diff prepare passthrough, got %#v", got)
	}
}
