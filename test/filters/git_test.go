package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestGitSummaries(t *testing.T) {
	if got := filters.SummarizeGitStatus(""); got != "clean" {
		t.Fatalf("unexpected clean status: %q", got)
	}
	status := filters.SummarizeGitStatus("## main\nM  a\n A b\n?? c\nx\n")
	if !strings.Contains(status, "staged=1 unstaged=1 untracked=1") {
		t.Fatalf("unexpected status summary: %q", status)
	}
	status = filters.SummarizeGitStatus("M  a\nM  b\nM  c\nM  d\nM  e\nM  f\nM  g\n")
	if strings.Count(status, "\n  ") != 6 {
		t.Fatalf("expected file preview to cap at 6 entries: %q", status)
	}

	if got := filters.SummarizeGitLog(""); got != "no commits" {
		t.Fatalf("unexpected empty git log: %q", got)
	}
	log := filters.SummarizeGitLog(strings.Repeat("hash subject\n", 12))
	if !strings.HasPrefix(log, "12 commits\n") {
		t.Fatalf("unexpected git log summary: %q", log)
	}

	if got := filters.SummarizeGitDiff(""); got != "no diff" {
		t.Fatalf("unexpected empty diff: %q", got)
	}
	diffStat := filters.SummarizeGitDiff(strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"+++ b/a.go",
		"--- a/a.go",
		"+foo",
		"-bar",
		" a.go | 2 +-",
		" 1 file changed, 1 insertion(+), 1 deletion(-)",
	}, "\n"))
	if !strings.Contains(diffStat, "files=1 +1 -1") || !strings.Contains(diffStat, "1 file changed") {
		t.Fatalf("unexpected diff stat: %q", diffStat)
	}
	diffFallback := filters.SummarizeGitDiff("diff --git a/a b/a\n+foo\n-bar")
	if !strings.Contains(diffFallback, "... +") && !strings.Contains(diffFallback, "diff --git") {
		t.Fatalf("unexpected diff fallback: %q", diffFallback)
	}
	diffLong := filters.SummarizeGitDiff(strings.Join([]string{
		"diff --git a/a b/a",
		" one | 1 +",
		" two | 1 +",
		" three | 1 +",
		" four | 1 +",
		" five | 1 +",
		" six | 1 +",
		" seven | 1 +",
		" eight | 1 +",
		" nine | 1 +",
	}, "\n"))
	if strings.Count(diffLong, "\n") != 8 {
		t.Fatalf("expected truncated diff summary, got %q", diffLong)
	}
}
