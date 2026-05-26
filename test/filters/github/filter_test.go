package github_test

import (
	"strings"
	"testing"

	ghfilter "github.com/devr-tools/szr/internal/filters/github"
)

func TestSummarizeGHPRView(t *testing.T) {
	t.Parallel()

	input := `{"number":42,"title":"Tighten reducers","state":"OPEN","isDraft":false,"headRefName":"feature/reducers","baseRefName":"main","reviewDecision":"CHANGES_REQUESTED","files":[{"path":"internal/filters/github.go","additions":80,"deletions":0},{"path":"internal/profiles/github.go","additions":60,"deletions":0}]}`
	got := ghfilter.SummarizeGHPRView(input, 5)
	for _, want := range []string{
		"PR #42 Tighten reducers state=open draft=false",
		"feature/reducers -> main review=changes_requested",
		"files: 2",
		"internal/filters/github.go +80 -0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh pr view summary:\n%s", want, got)
		}
	}
}

func TestSummarizeGHRunView(t *testing.T) {
	t.Parallel()

	input := `{"workflowName":"CI","status":"completed","conclusion":"failure","event":"push","headBranch":"main","url":"https://example.test/run/1","jobs":[{"name":"test","status":"completed","conclusion":"failure","steps":[{"name":"setup","conclusion":"success"},{"name":"unit","conclusion":"failure"}]},{"name":"lint","status":"completed","conclusion":"success","steps":[{"name":"eslint","conclusion":"success"}]}]}`
	got := ghfilter.SummarizeGHRunView(input, 6)
	for _, want := range []string{
		"CI status=completed conclusion=failure",
		"branch=main event=push",
		"job test status=completed conclusion=failure",
		"step unit",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh run view summary:\n%s", want, got)
		}
	}
}

func TestSummarizeGHRunLog(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"test\tSet up job\tRunner provisioning failed",
		"test\tUnit\tError: assertion failed",
		"test\tUnit\tError: assertion failed",
		"lint\tESLint\tfatal: unexpected token",
	}, "\n")
	got := ghfilter.SummarizeGHRunLog(input, 6)
	for _, want := range []string{
		"jobs_with_failures: 2",
		"test: Unit Error: assertion failed (x2)",
		"lint: ESLint fatal: unexpected token",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh run log summary:\n%s", want, got)
		}
	}
}

func TestGHRecoveryInfo(t *testing.T) {
	t.Parallel()

	prInput := `{"number":42,"title":"Tighten reducers","state":"OPEN","isDraft":false,"headRefName":"feature/reducers","baseRefName":"main","reviewDecision":"CHANGES_REQUESTED","files":[{"path":"internal/filters/github.go","additions":80,"deletions":0},{"path":"internal/profiles/github.go","additions":60,"deletions":0},{"path":"test/github_test.go","additions":20,"deletions":1}]}`
	if kind, summary, requireRawCapture := ghfilter.GHPRViewRecoveryInfo(prInput, 4); kind != "full-output" || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh pr recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	runViewInput := `{"workflowName":"CI","status":"completed","conclusion":"failure","event":"push","headBranch":"main","url":"https://example.test/run/1","jobs":[{"name":"test","status":"completed","conclusion":"failure","steps":[{"name":"setup","conclusion":"success"},{"name":"unit","conclusion":"failure"},{"name":"integration","conclusion":"failure"}]},{"name":"lint","status":"completed","conclusion":"failure","steps":[{"name":"eslint","conclusion":"failure"}]}]}`
	if kind, summary, requireRawCapture := ghfilter.GHRunViewRecoveryInfo(runViewInput, 4); kind != "full-output" || summary != "omitted 4 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected gh run view recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	runLogInput := strings.Join([]string{
		"test\tSet up job\tRunner provisioning failed",
		"test\tUnit\tError: assertion failed",
		"test\tIntegration\tError: timeout",
		"lint\tESLint\tfatal: unexpected token",
	}, "\n")
	if kind, summary, requireRawCapture := ghfilter.GHRunLogRecoveryInfo(runLogInput, 3); kind != "full-output" || summary != "omitted 2 additional log lines" || !requireRawCapture {
		t.Fatalf("unexpected gh run log recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}
