package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestBuildSystemProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "build-system")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"FAILED: app",
			"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
			"ninja: build stopped: subcommand failed.",
		}, "\n"),
	})
	for _, want := range []string{"FAILED: app", "src/app.cpp:12:3: error: use of undeclared identifier 'boom'", "ninja: build stopped: subcommand failed."} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in build-system render output:\n%s", want, rendered)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected build-system stream metadata: %#v", profile)
	}
}

func TestBuildSystemProfileStreamRecovery(t *testing.T) {
	list := profiles.Builtins(3)
	profile := testutil.FindProfile(t, list, "build-system")

	stream := profile.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"FAILED: src/app.cpp.o",
		"src/app.cpp:12:3: error: use of undeclared identifier 'boom'",
		"src/app.cpp:14:2: note: candidate function not viable",
		"src/lib.cpp:20:7: error: no member named 'x' in 'Thing'",
		"ninja: build stopped: subcommand failed.",
	}, "\n")))

	if got := stream.Result(); got == "" {
		t.Fatal("expected build-system stream output")
	}
	recoveryStream, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable build-system reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recoveryStream.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected build-system stream recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
