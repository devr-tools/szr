package profiles_test

import (
	"strings"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestRipgrepProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "ripgrep")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"one.go:1:first",
			"one.go:2:second",
			"two.go:9:two",
		}, "\n"),
	})
	for _, want := range []string{"one.go (2 matches)", "two.go (1 matches)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ripgrep render output:\n%s", want, rendered)
		}
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("a.go:1:hit\n"))
	streamed.ConsumeStdout([]byte("a.go:2:second\n"))
	if got := streamed.Result(); !strings.Contains(got, "a.go (2 matches)") {
		t.Fatalf("unexpected ripgrep stream output: %q", got)
	}
}
