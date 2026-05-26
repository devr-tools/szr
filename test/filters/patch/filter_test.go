package patch_test

import (
	"strings"
	"testing"

	patchfilter "github.com/devr-tools/szr/internal/filters/patch"
)

func TestSummarizePatchDiff(t *testing.T) {
	input := strings.Join([]string{
		"diff --git a/src/app.go b/src/app.go",
		"--- a/src/app.go",
		"+++ b/src/app.go",
		"@@ -1,2 +1,2 @@",
		"@@ -10,2 +10,2 @@",
		"error: patch failed: src/app.go:10",
		"src/app.go.rej",
	}, "\n")

	got := patchfilter.SummarizePatchDiff(input, 6)
	for _, want := range []string{
		"files=",
		"hunks=2",
		"diff --git a/src/app.go b/src/app.go",
		"error: patch failed: src/app.go:10",
		"src/app.go.rej",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in patch summary:\n%s", want, got)
		}
	}
}

func TestPatchDiffRecoveryInfo(t *testing.T) {
	input := strings.Join([]string{
		"diff --git a/src/app.go b/src/app.go",
		"--- a/src/app.go",
		"+++ b/src/app.go",
		"error: patch failed: src/app.go:10",
		"src/app.go.rej",
	}, "\n")

	if kind, summary, requireRawCapture := patchfilter.PatchDiffRecoveryInfo(input, 3); kind != "full-output" || summary != "omitted 3 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected patch recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
