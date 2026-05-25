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
