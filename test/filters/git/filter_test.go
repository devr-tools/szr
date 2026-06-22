package git_test

import (
	"strings"
	"testing"

	gitfilter "github.com/devr-tools/szr/internal/filters/git"
)

func TestGitSummaries(t *testing.T) {
	t.Run("status", testGitStatusSummary)
	t.Run("log", testGitLogSummary)
	t.Run("diff", testGitDiffSummary)
	t.Run("reducers", testGitReducers)
}

func testGitStatusSummary(t *testing.T) {
	t.Helper()
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "", want: "clean"},
		{input: "## main\n", want: "main"},
		{input: "## main\nM  a\n", want: "main\nM  a"},
	} {
		if got := gitfilter.SummarizeGitStatus(tc.input); got != tc.want {
			t.Fatalf("unexpected git status summary: %q", got)
		}
	}
	assertContainsAll(t, gitfilter.SummarizeGitStatus("## main\nM  a\n A b\n?? c\nx\n"), "staged=1 unstaged=1 untracked=1", "staged: a", "unstaged: b", "untracked: c")
	assertContainsAll(t, gitfilter.SummarizeGitStatus(" M internal/cli/app.go\n M internal/cli/help.go\n?? docs/bench/one.md\n?? docs/bench/two.md\n"), "unstaged: internal/... (2)", "untracked: docs/... (2)")
	if got := gitfilter.SummarizeGitStatus("M  a\nM  b\nM  c\nM  d\nM  e\nM  f\nM  g\n"); !strings.Contains(got, "... +4 more") {
		t.Fatalf("expected grouped status preview truncation: %q", got)
	}
}

func testGitLogSummary(t *testing.T) {
	t.Helper()
	if got := gitfilter.SummarizeGitLog(""); got != "no commits" {
		t.Fatalf("unexpected empty git log: %q", got)
	}
	assertContainsAll(t, gitfilter.SummarizeGitLog(strings.Repeat("hash subject\n", 12)), "12 commits", "hash subject (x12)")
	if got := gitfilter.SummarizeGitLog("a1 save\na2 save\na3 feat\n"); got != "3 commits\na1 save (x2)\na3 feat" {
		t.Fatalf("expected repeated git subjects to fold, got %q", got)
	}
}

func testGitDiffSummary(t *testing.T) {
	t.Helper()
	if got := gitfilter.SummarizeGitDiff(""); got != "no diff" {
		t.Fatalf("unexpected empty diff: %q", got)
	}
	assertContainsAll(t, gitfilter.SummarizeGitDiff(strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"+++ b/a.go",
		"--- a/a.go",
		"+foo",
		"-bar",
		" a.go | 2 +-",
		" 1 file changed, 1 insertion(+), 1 deletion(-)",
	}, "\n")), "files=1 +1 -1", "1 file changed")
	if got := gitfilter.SummarizeGitDiff("diff --git a/a b/a\n+foo\n-bar"); !strings.Contains(got, "a  +1 -1") {
		t.Fatalf("unexpected diff fallback: %q", got)
	}
	assertContainsAll(t, gitfilter.SummarizeGitDiff(strings.Join([]string{
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
	}, "\n")), "files=1 +3 -1", "internal/parser/lexer.go", "hunks=2", "func lex(input string) []token {", "func parse(tokens []token) node {")
	assertContainsAll(t, gitfilter.SummarizeGitDiff(strings.Join([]string{
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
	}, "\n")), "files=1 +0 -0", "eight | 1 +", "five | 1 +", "... +4 more files")
}

func testGitReducers(t *testing.T) {
	t.Helper()
	assertGitStatusReducer(t)
	assertGitLogReducer(t)
	assertGitDiffReducers(t)
}

func assertGitStatusReducer(t *testing.T) {
	t.Helper()
	statusReducer := gitfilter.NewGitStatusReducer(8, 0)
	statusReducer.ConsumeStdout([]byte("\x1b[32m## main\x1b[0m\nM  a\n"))
	statusReducer.ConsumeStdout([]byte("?? b\n"))
	statusStream := statusReducer.Result()
	if statusStream != "main\nM  a\n?? b" {
		t.Fatalf("unexpected compact git status stream: %q", statusStream)
	}
	if statusReducer.BytesParsed() == 0 || statusReducer.FallbackUsed() {
		t.Fatalf("unexpected git status reducer metadata: bytes=%d fallback=%v", statusReducer.BytesParsed(), statusReducer.FallbackUsed())
	}
	if preview := statusReducer.Preview(); preview != statusStream {
		t.Fatalf("expected git status preview to match result, got %q", preview)
	}
}

func assertGitLogReducer(t *testing.T) {
	t.Helper()
	logReducer := gitfilter.NewGitLogReducer(4, 0)
	logReducer.ConsumeStdout([]byte("one\n"))
	logReducer.ConsumeStdout([]byte("two\nthree\nfour\n"))
	if got := logReducer.Result(); got != "4 commits\none\ntwo\n... +2 more commits" {
		t.Fatalf("unexpected git log stream: %q", got)
	}
	if logReducer.BytesParsed() == 0 || logReducer.FallbackUsed() {
		t.Fatalf("unexpected git log reducer metadata: bytes=%d fallback=%v", logReducer.BytesParsed(), logReducer.FallbackUsed())
	}
	if preview := logReducer.Preview(); !strings.Contains(preview, "4 commits") {
		t.Fatalf("expected git log preview to be populated, got %q", preview)
	}
	if kind, summary, requireRawCapture := logReducer.RecoveryInfo(); kind != "full-output" || summary != "omitted 2 commits" || !requireRawCapture {
		t.Fatalf("unexpected git log recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func assertGitDiffReducers(t *testing.T) {
	t.Helper()
	diffReducer := gitfilter.NewGitDiffReducer(4, 0)
	diffReducer.ConsumeStdout([]byte("diff --git a/a.go b/a.go\n"))
	diffReducer.ConsumeStdout([]byte(" a.go | 2 +-\n+foo\n-bar\n"))
	assertContainsAll(t, diffReducer.Result(), "files=1 +1 -1", "a.go | 2 +-")
	if diffReducer.BytesParsed() == 0 || diffReducer.FallbackUsed() {
		t.Fatalf("unexpected git diff reducer metadata: bytes=%d fallback=%v", diffReducer.BytesParsed(), diffReducer.FallbackUsed())
	}
	if preview := diffReducer.Preview(); !strings.Contains(preview, "files=1 +1 -1") {
		t.Fatalf("expected git diff preview to be populated, got %q", preview)
	}
	if kind, summary, requireRawCapture := diffReducer.RecoveryInfo(); kind != "full-output" || summary != "omitted full diff hunks" || !requireRawCapture {
		t.Fatalf("unexpected git diff recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	patchReducer := gitfilter.NewGitDiffReducer(4, 0)
	patchReducer.ConsumeStdout([]byte("diff --git a/a.go b/a.go\n"))
	patchReducer.ConsumeStdout([]byte("@@ -1,2 +1,3 @@ func demo() {\n+foo\n-bar\n"))
	if got := patchReducer.Result(); !strings.Contains(got, "a.go  hunks=1  +1 -1  func demo() {") {
		t.Fatalf("unexpected git diff patch summary: %q", got)
	}

	stderrReducer := gitfilter.NewGitDiffReducer(4, 0)
	stderrReducer.ConsumeStderr([]byte("diff --git a/b.go b/b.go\n b.go | 1 +\n"))
	if got := stderrReducer.Result(); !strings.Contains(got, "b.go | 1 +") {
		t.Fatalf("expected stderr diff reducer to summarize stderr chunks, got %q", got)
	}

	fallbackReducer := gitfilter.NewGitDiffReducer(4, 0)
	fallbackReducer.ConsumeStdout([]byte("plain line\nanother line\n"))
	if !fallbackReducer.FallbackUsed() {
		t.Fatal("expected git diff reducer fallback when no diff markers are present")
	}
	if preview := fallbackReducer.Preview(); preview != "" {
		t.Fatalf("expected empty git diff preview without diff metadata, got %q", preview)
	}
	if kind, summary, requireRawCapture := fallbackReducer.RecoveryInfo(); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("expected no git diff recovery info for fallback-only reducer, got kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func assertContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}
