package git_test

import (
	"strings"
	"testing"

	gitfilter "github.com/devr-tools/szr/internal/filters/git"
)

func TestGitSummaries(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		if got := gitfilter.SummarizeGitStatus(""); got != "clean" {
			t.Fatalf("unexpected clean status: %q", got)
		}
		if got := gitfilter.SummarizeGitStatus("## main\n"); got != "main" {
			t.Fatalf("unexpected compact clean branch status: %q", got)
		}
		if got := gitfilter.SummarizeGitStatus("## main\nM  a\n"); got != "main\nM  a" {
			t.Fatalf("unexpected compact single-file status: %q", got)
		}
		status := gitfilter.SummarizeGitStatus("## main\nM  a\n A b\n?? c\nx\n")
		assertContainsAll(t, status, "staged=1 unstaged=1 untracked=1", "staged: a", "unstaged: b", "untracked: c")

		grouped := gitfilter.SummarizeGitStatus(" M internal/cli/app.go\n M internal/cli/help.go\n?? docs/bench/one.md\n?? docs/bench/two.md\n")
		assertContainsAll(t, grouped, "unstaged: internal/... (2)", "untracked: docs/... (2)")

		status = gitfilter.SummarizeGitStatus("M  a\nM  b\nM  c\nM  d\nM  e\nM  f\nM  g\n")
		if !strings.Contains(status, "... +4 more") {
			t.Fatalf("expected grouped status preview truncation: %q", status)
		}
	})

	t.Run("log", func(t *testing.T) {
		if got := gitfilter.SummarizeGitLog(""); got != "no commits" {
			t.Fatalf("unexpected empty git log: %q", got)
		}
		log := gitfilter.SummarizeGitLog(strings.Repeat("hash subject\n", 12))
		assertContainsAll(t, log, "12 commits", "hash subject (x12)")

		log = gitfilter.SummarizeGitLog("a1 save\na2 save\na3 feat\n")
		if log != "3 commits\na1 save (x2)\na3 feat" {
			t.Fatalf("expected repeated git subjects to fold, got %q", log)
		}
	})

	t.Run("diff", func(t *testing.T) {
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
		assertContainsAll(t, diffStat, "files=1 +1 -1", "1 file changed")

		diffFallback := gitfilter.SummarizeGitDiff("diff --git a/a b/a\n+foo\n-bar")
		if !strings.Contains(diffFallback, "a  +1 -1") {
			t.Fatalf("unexpected diff fallback: %q", diffFallback)
		}

		perFilePatch := gitfilter.SummarizeGitDiff(strings.Join([]string{
			"diff --git a/internal/parser/lexer.go b/internal/parser/lexer.go",
			"index 1234567..89abcde 100644",
			"--- a/internal/parser/lexer.go",
			"+++ b/internal/parser/lexer.go",
			"@@ -10,7 +10,16 @@ func lex(input string) []token {",
			"-\treturn nil",
			"+\tbuf := make([]token, 0, 128)",
			"+\treturn compact(buf)",
			"@@ -42,6 +42,10 @@ func parse(tokens []token) node {",
			"+\treturn root",
		}, "\n"))
		assertContainsAll(t, perFilePatch, "files=1 +3 -1", "internal/parser/lexer.go", "hunks=2", "func lex(input string) []token {", "func parse(tokens []token) node {")

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
		assertContainsAll(t, diffLong, "files=1 +0 -0", "eight | 1 +", "five | 1 +", "... +4 more files")
	})

	t.Run("reducers", func(t *testing.T) {
		statusReducer := gitfilter.NewGitStatusReducer(8, 0)
		statusReducer.ConsumeStdout([]byte("\x1b[32m## main\x1b[0m\nM  a\n"))
		statusReducer.ConsumeStdout([]byte("?? b\n"))
		statusStream := statusReducer.Result()
		if statusStream != "main\nM  a\n?? b" {
			t.Fatalf("unexpected compact git status stream: %q", statusStream)
		}

		logReducer := gitfilter.NewGitLogReducer(4, 0)
		logReducer.ConsumeStdout([]byte("one\n"))
		logReducer.ConsumeStdout([]byte("two\nthree\nfour\n"))
		if got := logReducer.Result(); got != "4 commits\none\ntwo\n... +2 more commits" {
			t.Fatalf("unexpected git log stream: %q", got)
		}

		diffReducer := gitfilter.NewGitDiffReducer(4, 0)
		diffReducer.ConsumeStdout([]byte("diff --git a/a.go b/a.go\n"))
		diffReducer.ConsumeStdout([]byte(" a.go | 2 +-\n+foo\n-bar\n"))
		diffStream := diffReducer.Result()
		assertContainsAll(t, diffStream, "files=1 +1 -1", "a.go | 2 +-")

		patchReducer := gitfilter.NewGitDiffReducer(4, 0)
		patchReducer.ConsumeStdout([]byte("diff --git a/a.go b/a.go\n"))
		patchReducer.ConsumeStdout([]byte("@@ -1,2 +1,3 @@ func demo() {\n+foo\n-bar\n"))
		if got := patchReducer.Result(); !strings.Contains(got, "a.go  hunks=1  +1 -1  func demo() {") {
			t.Fatalf("unexpected git diff patch summary: %q", got)
		}
	})
}

func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}
