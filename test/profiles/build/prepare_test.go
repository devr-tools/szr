package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBuildSystemProfilePrepare(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "build-system")
	advanced := config.Default().Advanced

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
	if profile.Match(engine.Invocation{Display: []string{"docker", "run", "alpine"}}) {
		t.Fatal("did not expect docker run to match build-system")
	}
	for _, tc := range []struct {
		command []string
		want    []string
	}{
		{[]string{"make", "test"}, []string{"make", "test", "--no-print-directory"}},
		{[]string{"bazel", "test", "//..."}, []string{"bazel", "test", "//...", "--noshow_progress", "--color=no", "--curses=no"}},
		{[]string{"terraform", "plan"}, []string{"terraform", "plan", "-no-color"}},
		{[]string{"tofu", "apply"}, []string{"tofu", "apply", "-no-color"}},
		{[]string{"helm", "upgrade", "app"}, []string{"helm", "upgrade", "app", "-no-color"}},
		{[]string{"gradle", "test"}, []string{"gradle", "test", "--quiet", "--console=plain"}},
		{[]string{"mvn", "test"}, []string{"mvn", "test", "--quiet", "--batch-mode"}},
		{[]string{"make", "test", "--silent"}, []string{"make", "test", "--silent"}},
		{[]string{"bazel", "test", "//...", "--noshow_progress", "--color=yes", "--curses=yes"}, []string{"bazel", "test", "//...", "--noshow_progress", "--color=yes", "--curses=yes"}},
		{[]string{"gradle", "test", "--info", "--console=rich"}, []string{"gradle", "test", "--info", "--console=rich"}},
		{[]string{"mvn", "test", "--errors", "-B"}, []string{"mvn", "test", "--errors", "-B"}},
	} {
		if got := profile.Prepare(engine.Invocation{Command: tc.command, Advanced: advanced}); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("unexpected build-system prepare for %#v: got %#v want %#v", tc.command, got, tc.want)
		}
	}

	if got := profile.Prepare(engine.Invocation{Command: []string{"make", "test"}}); !reflect.DeepEqual(got, []string{"make", "test"}) {
		t.Fatalf("expected non-aggressive build prepare passthrough, got %#v", got)
	}
}
