package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCPPProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	ctest := testutil.FindProfile(t, list, "ctest")
	rendered := ctest.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"Test project /tmp/build",
			"1/2 Test #1: api_smoke ....................***Failed    0.02 sec",
			"The following tests FAILED:",
			"1 - api_smoke (Failed)",
		}, "\n"),
	})
	for _, want := range []string{"api_smoke", "The following tests FAILED:"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ctest render output:\n%s", want, rendered)
		}
	}

	clang := testutil.FindProfile(t, list, "clang-tooling")
	streamed := clang.StreamRender(engine.Invocation{}, clang.Budget)
	streamed.ConsumeStderr([]byte("src/main.cpp:10:5: warning: use nullptr [modernize-use-nullptr]\n"))
	streamed.ConsumeStdout([]byte("bear: compiled 12 translation units\n"))
	got := streamed.Result()
	for _, want := range []string{"src/main.cpp:10:5: warning: use nullptr", "bear: compiled 12 translation units"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in clang-tooling stream output:\n%s", want, got)
		}
	}
}
