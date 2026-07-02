package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
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
	for _, want := range []string{"one.go:1: first (2 matches)", "two.go:9: two (1 matches)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in ripgrep render output:\n%s", want, rendered)
		}
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("a.go:1:hit\n"))
	streamed.ConsumeStdout([]byte("a.go:2:second\n"))
	streamed.ConsumeStdout([]byte("node_modules/pkg/a.go:4:ignored\n"))
	if got := streamed.Result(); !strings.Contains(got, "a.go:1: hit (2 matches)") {
		t.Fatalf("unexpected ripgrep stream output: %q", got)
	}
	if got := streamed.Result(); !strings.Contains(got, "suppressed noisy paths") {
		t.Fatalf("expected ripgrep stream suppression note, got %q", got)
	}
}

func TestFindProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "path-find")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"/tmp/z.py",
			"/tmp/a.py",
			"/tmp/m.py",
		}, "\n"),
	})
	for _, want := range []string{"3 matches", "/tmp/a.py", "/tmp/m.py"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in find render output:\n%s", want, rendered)
		}
	}

	streamed := profile.StreamRender(engine.Invocation{}, profile.Budget)
	streamed.ConsumeStdout([]byte("/tmp/a.py\n"))
	streamed.ConsumeStdout([]byte("/tmp/b.py\n"))
	streamed.ConsumeStdout([]byte("/tmp/node_modules/c.py\n"))
	if got := streamed.Result(); !strings.Contains(got, "2 matches") {
		t.Fatalf("unexpected find stream output: %q", got)
	}
	if got := streamed.Result(); !strings.Contains(got, "suppressed noisy paths") {
		t.Fatalf("expected find stream suppression note, got %q", got)
	}
}
