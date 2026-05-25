package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBuildSystemProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "build-system")

	for _, display := range [][]string{
		{"make", "test"},
		{"just", "build"},
		{"task", "lint"},
		{"bazel", "test", "//..."},
		{"ninja"},
		{"cmake", "--build", "build"},
		{"terraform", "plan"},
		{"tofu", "apply"},
		{"helm", "upgrade", "app"},
		{"gradle", "test"},
		{"mvn", "test"},
		{"docker", "build", "."},
		{"docker", "buildx", "build", "."},
	} {
		if !profile.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match build-system", display)
		}
	}
	if profile.Match(engine.Invocation{Display: []string{"ctest"}}) {
		t.Fatal("did not expect ctest to match build-system")
	}
	if got := profile.Prepare(engine.Invocation{Command: []string{"make", "test"}}); !reflect.DeepEqual(got, []string{"make", "test"}) {
		t.Fatalf("expected build-system prepare passthrough, got %#v", got)
	}
}
