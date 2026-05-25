package profiles_test

import (
	"reflect"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCPPProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	ctest := testutil.FindProfile(t, list, "ctest")
	if !ctest.Match(engine.Invocation{Display: []string{"ctest", "-R", "api"}}) {
		t.Fatal("expected ctest to match")
	}
	if got := ctest.Prepare(engine.Invocation{Command: []string{"ctest"}}); !reflect.DeepEqual(got, []string{"ctest", "--output-on-failure"}) {
		t.Fatalf("unexpected ctest prepare: %#v", got)
	}
	if got := ctest.Prepare(engine.Invocation{Command: []string{"ctest", "--output-on-failure"}}); !reflect.DeepEqual(got, []string{"ctest", "--output-on-failure"}) {
		t.Fatalf("expected explicit ctest output flag to be preserved: %#v", got)
	}

	clang := testutil.FindProfile(t, list, "clang-tooling")
	for _, display := range [][]string{
		{"clang-tidy", "src/main.cpp"},
		{"clang-format", "--dry-run", "src/main.cpp"},
		{"bear", "--", "make"},
	} {
		if !clang.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected %#v to match clang-tooling", display)
		}
	}
	if got := clang.Prepare(engine.Invocation{Command: []string{"clang-tidy", "src/main.cpp"}}); !reflect.DeepEqual(got, []string{"clang-tidy", "src/main.cpp"}) {
		t.Fatalf("expected clang-tooling prepare passthrough, got %#v", got)
	}
}
