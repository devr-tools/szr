package git_test

import (
	"fmt"
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

	fullFormat := strings.Join([]string{
		"commit 0472258b6eacef6a79c7758134b036a960b88722",
		"Author: Arena <arena@example.com>",
		"Date:   Thu Jul 2 18:12:21 2026 -0400",
		"",
		"    fix: correct arena fixture failures",
		"",
		"commit 64c323b9ad5010255314394cae6ac21ed63b720b",
		"Author: Arena <arena@example.com>",
		"Date:   Thu Jul 2 18:11:10 2026 -0400",
		"",
		"    chore: arena revision 80",
		"",
		"commit 67933300925e3119044f97bbe5c008845dfe8382",
		"Author: Arena <arena@example.com>",
		"Date:   Thu Jul 2 18:11:10 2026 -0400",
		"",
		"    chore: arena revision 79",
		"",
	}, "\n")
	if got := gitfilter.SummarizeGitLog(fullFormat); got != "3 commits\n0472258 fix: correct arena fixture failures\n64c323b chore: arena revision 80\n... +1 more commits" {
		t.Fatalf("expected default-format git log to fold per commit, got %q", got)
	}
	expanded := gitfilter.NewGitLogReducerWithEntries(11, 0, 3)
	expanded.ConsumeStdout([]byte(fullFormat))
	assertContainsAll(t, expanded.Result(), "3 commits", "0472258 fix: correct arena fixture failures", "64c323b chore: arena revision 80", "6793330 chore: arena revision 79")
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

// TestGitDiffVerbatimSmallDiff pins the small-diff fidelity mode: a diff
// whose changed lines fit the verbatim caps replays every +/- line under its
// file header instead of collapsing to stats-only anchors.
func TestGitDiffVerbatimSmallDiff(t *testing.T) {
	t.Parallel()

	small := strings.Join([]string{
		"diff --git a/calc/history.go.txt b/calc/history.go.txt",
		"index 32dcca2..fffff18 100644",
		"--- a/calc/history.go.txt",
		"+++ b/calc/history.go.txt",
		"@@ -74,3 +74,7 @@",
		" // rev 74",
		"+// rev 77",
		"+// rev 80",
		"diff --git a/src/deep.go b/src/deep.go",
		"index c9dbc9a..52c0fd3 100644",
		"--- a/src/deep.go",
		"+++ b/src/deep.go",
		"@@ -1 +1,3 @@",
		"-deep marker",
		"+package deep",
	}, "\n")
	reducer := gitfilter.NewGitDiffReducer(12, 0)
	got := reducer.Reduce(small)
	assertContainsAll(t, got,
		"files=2 +3 -1",
		"calc/history.go.txt  hunks=1  +2 -0",
		"+// rev 77",
		"+// rev 80",
		"src/deep.go  hunks=1  +1 -1",
		"-deep marker",
		"+package deep",
	)
	if strings.Contains(got, "// rev 74") {
		t.Fatalf("expected context lines to stay omitted, got %q", got)
	}
	if strings.Contains(got, "index 32dcca2") || strings.Contains(got, "+++ b/") {
		t.Fatalf("expected index/filename noise to stay omitted, got %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != "full-output" || summary != "omitted diff context lines" || !requireRawCapture {
		t.Fatalf("unexpected small-diff recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

// TestGitDiffVerbatimSelfCapsUnderCompressionContract pins the self-capping
// mode: when the raw diff is big enough to arm the engine compression
// contract, the verbatim render fits itself into the predicted allowance —
// every filename stays (label-only headers), the cheapest changed lines
// survive first, and a bare "..." marks the dropped remainder.
func TestGitDiffVerbatimSelfCapsUnderCompressionContract(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	long := strings.Repeat("x", 60)
	for _, file := range []string{"alpha/first.go", "beta/second.go", "gamma/third.go"} {
		fmt.Fprintf(&builder, "diff --git a/%s b/%s\n", file, file)
		fmt.Fprintf(&builder, "index 1234567..89abcde 100644\n--- a/%s\n+++ b/%s\n", file, file)
		builder.WriteString("@@ -1,8 +1,8 @@ func anchor() {\n")
		for i := 0; i < 3; i++ {
			fmt.Fprintf(&builder, " context line %d padding padding padding padding\n", i)
		}
		builder.WriteString("+short\n")
		fmt.Fprintf(&builder, "+long change %s\n", long)
	}
	input := builder.String()
	if len(input) < 600 {
		t.Fatalf("fixture must arm the contract proxy, got %d bytes", len(input))
	}

	reducer := gitfilter.NewGitDiffReducer(12, 0)
	got := reducer.Reduce(input)
	assertContainsAll(t, got, "files=3 +6 -0", "alpha/first.go", "beta/second.go", "gamma/third.go", "+short", "\n...")
	if strings.Contains(got, "hunks=") || strings.Contains(got, "func anchor() {") {
		t.Fatalf("expected label-only headers in self-capped render, got %q", got)
	}
	if strings.Contains(got, long) {
		t.Fatalf("expected expensive changed lines to be dropped first, got %q", got)
	}
}

// TestGitDiffVerbatimCapsDisableOnLargeDiff pins the fallback: once a diff
// exceeds the verbatim line budget the reducer releases the retained lines
// and renders the existing stat/anchor summary.
func TestGitDiffVerbatimCapsDisableOnLargeDiff(t *testing.T) {
	t.Parallel()

	var builder strings.Builder
	builder.WriteString("diff --git a/big.go b/big.go\n")
	builder.WriteString("@@ -1,50 +1,50 @@ func big() {\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&builder, "+line %d added to overflow the verbatim budget\n", i)
	}
	reducer := gitfilter.NewGitDiffReducer(12, 0)
	got := reducer.Reduce(builder.String())
	assertContainsAll(t, got, "files=1 +50 -0", "big.go  hunks=1  +50 -0  func big() {")
	if strings.Contains(got, "+line 0 added") {
		t.Fatalf("expected large diff to drop verbatim lines, got %q", got)
	}
	if kind, summary, requireRawCapture := reducer.RecoveryInfo(); kind != "full-output" || summary != "omitted full diff hunks" || !requireRawCapture {
		t.Fatalf("unexpected large-diff recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	aggressive := gitfilter.NewGitDiffReducerWithOptions(gitfilter.GitDiffReducerOptions{MaxLines: 12, Aggressive: true})
	compact := aggressive.Reduce("diff --git a/a.go b/a.go\n@@ -1 +1 @@ func demo() {\n+foo\n-bar\n")
	if strings.Contains(compact, "+foo") {
		t.Fatalf("expected aggressive mode to skip verbatim hunks, got %q", compact)
	}
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

	conflictReducer := gitfilter.NewGitDiffReducer(4, 0)
	conflictReducer.ConsumeStdout([]byte("diff --cc conflicted.txt\nindex 065e9d1,8209b71..0000000\n--- a/conflicted.txt\n+++ b/conflicted.txt\n@@@ -1,1 -1,1 +1,5 @@@\n++<<<<<<< HEAD\n +main change\n++=======\n+ side change\n++>>>>>>> side\n"))
	assertContainsAll(t, conflictReducer.Result(), "files=1", "conflicted.txt [conflict]", "hunks=1")

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
