package profiles_test

import (
	"strings"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestPatchDiffProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "patch-diff")

	rendered := profile.Render(engine.Invocation{}, engine.Execution{
		Stdout: strings.Join([]string{
			"diff --git a/src/app.go b/src/app.go",
			"--- a/src/app.go",
			"+++ b/src/app.go",
			"@@ -1,2 +1,2 @@",
			"error: patch failed: src/app.go:10",
		}, "\n"),
	})
	for _, want := range []string{"files=", "hunks=1", "diff --git a/src/app.go b/src/app.go", "error: patch failed: src/app.go:10"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in patch-diff render output:\n%s", want, rendered)
		}
	}
	if profile.StreamPreference != engine.StreamStdoutFirst || profile.StreamRender == nil {
		t.Fatalf("unexpected patch-diff stream metadata: %#v", profile)
	}
}
