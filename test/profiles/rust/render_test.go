package profiles_test

import (
	"strings"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
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
