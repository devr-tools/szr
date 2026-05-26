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

func TestCPPProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	ctest := testutil.FindProfile(t, list, "ctest")
	ctestStream := ctest.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	ctestStream.ConsumeStdout([]byte(strings.Join([]string{
		"Test project /tmp/build",
		"1/2 Test #1: api_smoke ....................***Failed    0.02 sec",
		"src/api_test.cpp:19: Assertion failed",
		"The following tests FAILED:",
		"1 - api_smoke (Failed)",
	}, "\n")))
	ctestRecovery, ok := ctestStream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable ctest reducer, got %T", ctestStream)
	}
	if kind, summary, requireRawCapture := ctestRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected ctest recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	clang := testutil.FindProfile(t, list, "clang-tooling")
	clangStream := clang.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 1})
	clangStream.ConsumeStderr([]byte(strings.Join([]string{
		"src/main.cpp:10:5: warning: use nullptr [modernize-use-nullptr]",
		"include/app.h:7:2: error: expected ';' after class",
		"src/lib.cpp:20:7: warning: dead store [clang-analyzer-deadcode.DeadStores]",
		"bear: compiled 12 translation units",
	}, "\n")))
	clangRecovery, ok := clangStream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable clang reducer, got %T", clangStream)
	}
	if kind, summary, requireRawCapture := clangRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected clang recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
