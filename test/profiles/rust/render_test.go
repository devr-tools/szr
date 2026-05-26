package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCargoProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	cargoTest := testutil.FindProfile(t, list, "cargo-test")
	rendered := cargoTest.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"running 1 test",
			"test tests::smoke ... FAILED",
			"thread 'tests::smoke' panicked at src/lib.rs:8:5:",
			"assertion `left == right` failed",
			"test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out",
		}, "\n"),
	})
	for _, want := range []string{"test tests::smoke ... FAILED", "src/lib.rs:8:5", "test result: FAILED"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in cargo test render output:\n%s", want, rendered)
		}
	}
	if cargoTest.StreamPreference != engine.StreamStdoutFirst || cargoTest.StreamRender == nil {
		t.Fatalf("unexpected cargo test stream metadata: %#v", cargoTest)
	}

	cargoBuild := testutil.FindProfile(t, list, "cargo-build")
	streamed := cargoBuild.StreamRender(engine.Invocation{}, cargoBuild.Budget)
	streamed.ConsumeStderr([]byte("error[E0308]: mismatched types\n"))
	streamed.ConsumeStderr([]byte("--> src/main.rs:10:9\nhelp: try using `into()`\n"))
	got := streamed.Result()
	for _, want := range []string{"error[E0308]: mismatched types", "--> src/main.rs:10:9", "help: try using `into()`"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in cargo build streamed output:\n%s", want, got)
		}
	}
	if cargoBuild.StreamPreference != engine.StreamStderrFirst || cargoBuild.StreamRender == nil {
		t.Fatalf("unexpected cargo build stream metadata: %#v", cargoBuild)
	}
}

func TestCargoProfilesStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)

	cargoTest := testutil.FindProfile(t, list, "cargo-test")
	testStream := cargoTest.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	testStream.ConsumeStdout([]byte(strings.Join([]string{
		"test tests::math::subtracts ... FAILED",
		"thread 'tests::math::subtracts' panicked at src/lib.rs:42:5:",
		"assertion `left == right` failed",
		"test result: FAILED. 2 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out",
		"error: test failed, to rerun pass `--lib`",
	}, "\n")))
	testRecovery, ok := testStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable cargo test reducer, got %T", testStream)
	}
	if kind, summary, requireRawCapture := testRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected cargo test recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	cargoBuild := testutil.FindProfile(t, list, "cargo-build")
	buildStream := cargoBuild.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	buildStream.ConsumeStderr([]byte(strings.Join([]string{
		"error[E0432]: unresolved import `missing::Thing`",
		"--> src/lib.rs:4:5",
		"help: consider importing this module instead",
		"warning: unused import: `std::fmt`",
		"error: could not compile `app` due to 1 previous error; 1 warning emitted",
	}, "\n")))
	buildRecovery, ok := buildStream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable cargo build reducer, got %T", buildStream)
	}
	if kind, summary, requireRawCapture := buildRecovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected cargo build recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
