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
		"#42 OPEN Tighten reducers [review:changes_requested] feature/reducers->main",
		"files: 2",
		"internal/filters/github.go +80 -0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in gh pr view summary:\n%s", want, got)
		}
	}
	if !strings.HasPrefix(got, "#42 OPEN Tighten reducers") {
		t.Fatalf("expected PR headline as the first render line, got:\n%s", got)
	}
}

// TestSummarizeGHPRViewCapsFileList pins the self-capping behavior: the file
// list folds into "+N more files" within the line budget so the compression
// contract never has to crush the render (and with it the title headline).
func TestSummarizeGHPRViewCapsFileList(t *testing.T) {
	t.Parallel()

	input := `{"number":45,"title":"feat: summarize raw gh api JSON responses","state":"OPEN","isDraft":false,"headRefName":"feat-gh-api","baseRefName":"main","reviewDecision":"","files":[` +
		`{"path":"internal/cli/spread.go","additions":15,"deletions":2},` +
		`{"path":"internal/history/summary.go","additions":7,"deletions":2},` +
		`{"path":"internal/history/types.go","additions":1,"deletions":0},` +
		`{"path":"internal/profiles/ghapi/profile.go","additions":120,"deletions":0},` +
		`{"path":"test/profiles/ghapi/render_test.go","additions":79,"deletions":2}]}`
	got := ghfilter.SummarizeGHPRView(input, 5)
	lines := strings.Split(got, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected render to fit the 5-line budget, got %d lines:\n%s", len(lines), got)
	}
	if lines[0] != "#45 OPEN feat: summarize raw gh api JSON responses [review:none] feat-gh-api->main" {
		t.Fatalf("unexpected PR headline: %q", lines[0])
	}
	if lines[1] != "files: 5" {
		t.Fatalf("unexpected files count line: %q", lines[1])
	}
	if lines[len(lines)-1] != "+3 more files" {
		t.Fatalf("expected trailing more-files marker, got %q", lines[len(lines)-1])
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
