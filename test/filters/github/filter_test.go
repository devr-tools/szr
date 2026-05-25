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
