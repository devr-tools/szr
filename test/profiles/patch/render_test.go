package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
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

func TestPatchDiffProfileStreamRecovery(t *testing.T) {
	list := profiles.Builtins(6)
	profile := testutil.FindProfile(t, list, "patch-diff")
	stream := profile.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 3})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"diff --git a/src/app.go b/src/app.go",
		"--- a/src/app.go",
		"+++ b/src/app.go",
		"error: patch failed: src/app.go:10",
		"src/app.go.rej",
	}, "\n")))

	recoveryStream, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable patch reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recoveryStream.RecoveryInfo(); kind != "full-output" || summary != "omitted 3 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected patch recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
