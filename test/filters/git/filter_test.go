package git_test

import (
	"strings"
	"testing"

	gitfilter "szr/internal/filters/git"
)

func TestGitSummaries(t *testing.T) {
	if got := gitfilter.SummarizeGitStatus(""); got != "clean" {
		t.Fatalf("unexpected clean status: %q", got)
	}
	status := gitfilter.SummarizeGitStatus("## main\nM  a\n A b\n?? c\nx\n")
	if !strings.Contains(status, "staged=1 unstaged=1 untracked=1") {
		t.Fatalf("unexpected status summary: %q", status)
	}
	if !strings.Contains(status, "staged: a") || !strings.Contains(status, "unstaged: b") || !strings.Contains(status, "untracked: c") {
		t.Fatalf("expected grouped status previews, got %q", status)
	}
	grouped := gitfilter.SummarizeGitStatus(" M internal/cli/app.go\n M internal/cli/help.go\n?? docs/bench/one.md\n?? docs/bench/two.md\n")
	for _, want := range []string{"unstaged: internal/... (2)", "untracked: docs/... (2)"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in grouped status preview %q", want, grouped)
		}
	}
	status = gitfilter.SummarizeGitStatus("M  a\nM  b\nM  c\nM  d\nM  e\nM  f\nM  g\n")
	if !strings.Contains(status, "... +4 more") {
		t.Fatalf("expected grouped status preview truncation: %q", status)
	}

	if got := gitfilter.SummarizeGitLog(""); got != "no commits" {
		t.Fatalf("unexpected empty git log: %q", got)
	}
	log := gitfilter.SummarizeGitLog(strings.Repeat("hash subject\n", 12))
	if !strings.Contains(log, "hash subject (x12)") {
		t.Fatalf("unexpected git log summary: %q", log)
	}
	log = gitfilter.SummarizeGitLog("a1 save\na2 save\na3 feat\n")
	if log != "a1 save (x2)\na3 feat" {
		t.Fatalf("expected repeated git subjects to fold, got %q", log)
	}

	if got := gitfilter.SummarizeGitDiff(""); got != "no diff" {
		t.Fatalf("unexpected empty diff: %q", got)
	}
	diffStat := gitfilter.SummarizeGitDiff(strings.Join([]string{
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
	diffFallback := gitfilter.SummarizeGitDiff("diff --git a/a b/a\n+foo\n-bar")
	if !strings.Contains(diffFallback, "... +") && !strings.Contains(diffFallback, "diff --git") {
		t.Fatalf("unexpected diff fallback: %q", diffFallback)
	}
	diffLong := gitfilter.SummarizeGitDiff(strings.Join([]string{
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

	statusReducer := gitfilter.NewGitStatusReducer(8, 0)
	statusReducer.ConsumeStdout([]byte("\x1b[32m## main\x1b[0m\nM  a\n"))
	statusReducer.ConsumeStdout([]byte("?? b\n"))
	statusStream := statusReducer.Result()
	if !strings.Contains(statusStream, "main") || !strings.Contains(statusStream, "staged=1 unstaged=0 untracked=1") {
		t.Fatalf("unexpected git status stream: %q", statusStream)
	}

	logReducer := gitfilter.NewGitLogReducer(4, 0)
	logReducer.ConsumeStdout([]byte("one\n"))
	logReducer.ConsumeStdout([]byte("two\nthree\nfour\n"))
	if got := logReducer.Result(); got != "one\ntwo\n... +2 more commits" {
		t.Fatalf("unexpected git log stream: %q", got)
	}

	diffReducer := gitfilter.NewGitDiffReducer(4, 0)
	diffReducer.ConsumeStdout([]byte("diff --git a/a.go b/a.go\n"))
	diffReducer.ConsumeStdout([]byte(" a.go | 2 +-\n+foo\n-bar\n"))
	diffStream := diffReducer.Result()
	if !strings.Contains(diffStream, "files=1 +1 -1") || !strings.Contains(diffStream, "a.go | 2 +-") {
		t.Fatalf("unexpected git diff stream: %q", diffStream)
	}
}
