package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

const ghChecksMixedTable = "CodeQL\tpass\t6s\thttps://github.com/devr-tools/szr/runs/84842210450\n" +
	"lint\tpass\t24s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420601\n" +
	"test (ubuntu-latest full)\tfail\t42s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696\n"

func TestGHPRChecksProfileRender(t *testing.T) {
	list := profiles.Builtins(6)
	ghChecks := testutil.FindProfile(t, list, "gh-pr-checks")

	rendered := ghChecks.Render(engine.Invocation{}, engine.Execution{Stdout: ghChecksMixedTable, ExitCode: 8})
	for _, want := range []string{
		"checks: 2 pass, 1 fail (3 total)",
		"fail: test (ubuntu-latest full) 42s https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in gh pr checks render output:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "https://github.com/devr-tools/szr/runs/84842210450") {
		t.Fatalf("expected passing check URL to be dropped:\n%s", rendered)
	}

	allPass := "CodeQL\tpass\t6s\thttps://github.com/devr-tools/szr/runs/84842210450\n" +
		"lint\tpass\t24s\thttps://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420601\n"
	if got := ghChecks.Render(engine.Invocation{}, engine.Execution{Stdout: allPass}); got != "checks: 2 pass (2 total)" {
		t.Fatalf("expected all-pass summary line only, got:\n%s", got)
	}
	if ghChecks.StreamPreference != engine.StreamStdoutFirst || ghChecks.StreamRender == nil {
		t.Fatalf("unexpected gh pr checks stream metadata: %#v", ghChecks)
	}
}

func TestGHPRChecksProfileStream(t *testing.T) {
	list := profiles.Builtins(6)
	ghChecks := testutil.FindProfile(t, list, "gh-pr-checks")

	stream := ghChecks.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 6})
	stream.ConsumeStdout([]byte(ghChecksMixedTable))
	stream.ConsumeStdout([]byte("\n" + ghChecksMixedTable))
	got := stream.Result()
	for _, want := range []string{
		"checks: 2 pass, 1 fail (3 total) (watched 1 updates)",
		"fail: test (ubuntu-latest full) 42s https://github.com/devr-tools/szr/actions/runs/28610684653/job/84842420696",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh pr checks streamed output:\n%s", want, got)
		}
	}

	recovery, ok := stream.(interface{ RecoveryInfo() (string, string, bool) })
	if !ok {
		t.Fatalf("expected recovery-capable gh pr checks reducer, got %T", stream)
	}
	// Two pass rows fold into the counts line and three repainted rows dedupe.
	if kind, summary, requireRawCapture := recovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 5 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh pr checks recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
