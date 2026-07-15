package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

// TestGlabPipelineProfile pins matching for the ci/pipeline listing
// spellings and a render that keeps every non-dominant status row while
// folding the dominant status into a count.
func TestGlabPipelineProfile(t *testing.T) {
	list := profiles.Builtins(6)
	glabPipeline := testutil.FindProfile(t, list, "glab-pipeline")

	for _, display := range [][]string{
		{"glab", "ci", "status"},
		{"glab", "ci", "list"},
		{"glab", "pipeline", "list"},
		{"glab", "-R", "acme/link-service", "ci", "list"},
	} {
		if !glabPipeline.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected glab-pipeline to match %v", display)
		}
	}
	for _, display := range [][]string{
		{"glab", "mr", "list"},
		{"glab", "ci", "trace"},
		{"glab"},
		{"gh", "pipeline", "list"},
	} {
		if glabPipeline.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected glab-pipeline not to match %v", display)
		}
	}

	rendered := glabPipeline.Render(engine.Invocation{}, engine.Execution{Stdout: strings.Join([]string{
		"Showing 6 pipelines on acme/link-service (Page 1)",
		"",
		"(success) • #106\tmain\t(about 2 hours ago)",
		"(failed) • #105\tfeat/retry-backoff\t(about 3 hours ago)",
		"(success) • #104\tmain\t(1 day ago)",
		"(success) • #103\tmain\t(1 day ago)",
		"(success) • #102\tmain\t(2 days ago)",
		"(success) • #101\tmain\t(2 days ago)",
	}, "\n")})
	for _, want := range []string{
		"pipelines: 6 (success=5 failed=1)",
		"failed: #105 feat/retry-backoff (3h)",
		"success: #106 main (2h)",
		"... +2 more success",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in glab pipeline render:\n%s", want, rendered)
		}
	}
}

// TestGlabCIStatusRender pins job-table handling: job rows aggregate by
// status and the trailing pipeline identity line survives as context.
func TestGlabCIStatusRender(t *testing.T) {
	list := profiles.Builtins(6)
	glabPipeline := testutil.FindProfile(t, list, "glab-pipeline")

	rendered := glabPipeline.Render(engine.Invocation{}, engine.Execution{Stdout: strings.Join([]string{
		"(success) • lint\tlint",
		"(failed) • unit-tests\ttest",
		"(success) • build\tbuild",
		"(running) • integration-tests\ttest",
		"Pipeline #418223 (running) for feat/retry-backoff by devbot (push)",
	}, "\n")})
	for _, want := range []string{
		"pipelines: 4 (success=2 failed=1 running=1)",
		"failed: unit-tests test",
		"Pipeline #418223 (running) for feat/retry-backoff by devbot (push)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in glab ci status render:\n%s", want, rendered)
		}
	}

	stream := glabPipeline.StreamRender(engine.Invocation{}, engine.OutputBudget{MaxLines: 10})
	stream.ConsumeStdout([]byte(strings.Join([]string{
		"(success) • #106\tmain\t(about 2 hours ago)",
		"(success) • #105\tmain\t(1 day ago)",
		"(success) • #104\tmain\t(1 day ago)",
		"(failed) • #103\tfeat/x\t(2 days ago)",
		"(success) • #102\tmain\t(2 days ago)",
	}, "\n")))
	recovery, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatalf("expected recovery-capable glab reducer, got %T", stream)
	}
	if kind, summary, requireRawCapture := recovery.RecoveryInfo(); kind != "full-output" || summary != "omitted 1 additional rows" || !requireRawCapture {
		t.Fatalf("unexpected glab recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
