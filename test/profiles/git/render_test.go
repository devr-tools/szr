package profiles_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestGitProfilesRender(t *testing.T) {
	list := profiles.Builtins(6)

	gitAdd := testutil.FindProfile(t, list, "git-add")
	assertGitAddRender(t, gitAdd)

	gitCommit := testutil.FindProfile(t, list, "git-commit")
	assertGitCommitRender(t, gitCommit)

	gitPush := testutil.FindProfile(t, list, "git-push")
	assertGitPushRender(t, gitPush)

	gitPull := testutil.FindProfile(t, list, "git-pull")
	assertGitPullRender(t, gitPull)

	gitShow := testutil.FindProfile(t, list, "git-show")
	assertGitShowRender(t, gitShow)

	gitStatus := testutil.FindProfile(t, list, "git-status")
	assertGitStatusRender(t, gitStatus)

	gitLog := testutil.FindProfile(t, list, "git-log")
	assertGitLogRender(t, gitLog)

	gitDiff := testutil.FindProfile(t, list, "git-diff")
	assertGitDiffRender(t, gitDiff)
}

func assertGitAddRender(t *testing.T, gitAdd engine.Profile) {
	t.Helper()
	inv := engine.Classify(engine.Invocation{Command: []string{"git", "add", "internal/profiles/git/profile.go"}})
	if got := gitAdd.Render(inv, engine.Execution{}); got != "staged internal/... (1)" {
		t.Fatalf("unexpected git-add render output: %q", got)
	}
	if gitAdd.StreamPreference != engine.StreamStdoutFirst || gitAdd.StreamRender == nil {
		t.Fatalf("unexpected git-add stream metadata: %#v", gitAdd)
	}
	stream := gitAdd.StreamRender(inv, gitAdd.Budget)
	stream.ConsumeStdout([]byte(""))
	if got := stream.Result(); got != "staged internal/... (1)" {
		t.Fatalf("unexpected git-add stream output: %q", got)
	}
}

func assertGitCommitRender(t *testing.T, gitCommit engine.Profile) {
	t.Helper()
	inv := engine.Classify(engine.Invocation{Command: []string{"git", "commit", "-m", "tighten reducer"}})
	if got := gitCommit.Render(inv, engine.Execution{Stdout: "[main abc1234] tighten reducer\n 2 files changed, 7 insertions(+), 1 deletion(-)\n"}); got != "committed abc1234 tighten reducer files=2 +7 -1" {
		t.Fatalf("unexpected git-commit render output: %q", got)
	}
}

func assertGitPushRender(t *testing.T, gitPush engine.Profile) {
	t.Helper()
	if got := gitPush.Render(engine.Invocation{}, engine.Execution{Stderr: "To github.com:devr-tools/szr.git\n   abc1234..def5678  main -> main\n"}); got != "pushed main abc1234..def5678" {
		t.Fatalf("unexpected git-push render output: %q", got)
	}
	if gitPush.StreamPreference != engine.StreamStderrFirst || gitPush.StreamRender == nil {
		t.Fatalf("unexpected git-push stream metadata: %#v", gitPush)
	}
	stream := gitPush.StreamRender(engine.Invocation{}, gitPush.Budget)
	stream.ConsumeStderr([]byte("To github.com:devr-tools/szr.git\n   abc1234..def5678  main -> main\n"))
	if got := stream.Result(); got != "pushed main abc1234..def5678" {
		t.Fatalf("unexpected git-push stream output: %q", got)
	}
	provider, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatal("expected git-push stream reducer recovery provider")
	}
	if kind, summary, requireRawCapture := provider.RecoveryInfo(); kind != "full-output" || summary != "omitted git push success details" || !requireRawCapture {
		t.Fatalf("unexpected git-push recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func assertGitPullRender(t *testing.T, gitPull engine.Profile) {
	t.Helper()
	stdout := "Updating abc1234..def5678\nFast-forward\n 1 file changed, 3 insertions(+)\n"
	if got := gitPull.Render(engine.Invocation{}, engine.Execution{Stdout: stdout}); got != "pulled abc1234..def5678 fast-forward files=1 +3 -0" {
		t.Fatalf("unexpected git-pull render output: %q", got)
	}
}

func assertGitShowRender(t *testing.T, gitShow engine.Profile) {
	t.Helper()

	statInv := engine.Classify(engine.Invocation{Command: []string{"git", "show", "--stat", "abc1234"}})
	statStdout := strings.Join([]string{
		"commit abc1234567890fedcba",
		" internal/profiles/git/profile.go | 12 +++++++++---",
		" test/profiles/git/render_test.go | 4 ++--",
		" 2 files changed, 10 insertions(+), 6 deletions(-)",
	}, "\n")
	if got := gitShow.Render(statInv, engine.Execution{Stdout: statStdout}); got == "" || !strings.Contains(got, "show abc123456789") || !strings.Contains(got, "files=2 +10 -6") {
		t.Fatalf("unexpected git-show stat render output: %q", got)
	}

	blobInv := engine.Classify(engine.Invocation{Command: []string{"git", "show", "HEAD:internal/profiles/git/profile.go"}})
	blobStdout := "package git\n\nfunc demo() {\n\tprintln(\"x\")\n}\n"
	if got := gitShow.Render(blobInv, engine.Execution{Stdout: blobStdout}); !strings.Contains(got, "1  package git") || !strings.Contains(got, "3  func demo() { ... }") {
		t.Fatalf("unexpected git-show blob render output: %q", got)
	}

	nameStatusInv := engine.Classify(engine.Invocation{Command: []string{"git", "show", "--name-status", "abc1234"}})
	nameStatusStdout := strings.Join([]string{
		"commit abc1234567890fedcba",
		"Author: Dev",
		"Date:   2026-06-21",
		"",
		"    tighten reducer coverage",
		"",
		"M\tinternal/profiles/git/profile.go",
		"A\ttest/profiles/git/render_test.go",
	}, "\n")
	if got := gitShow.Render(nameStatusInv, engine.Execution{Stdout: nameStatusStdout}); !strings.Contains(got, "show abc123456789") || !strings.Contains(got, "internal/profiles/git/profile.go") || !strings.Contains(got, "test/profiles/git/render_test.go") {
		t.Fatalf("unexpected git-show name-status render output: %q", got)
	}

	nameOnlyInv := engine.Classify(engine.Invocation{Command: []string{"git", "show", "--name-only", "abc1234"}})
	nameOnlyStdout := strings.Join([]string{
		"commit abc1234567890fedcba",
		"",
		"internal/profiles/git/profile.go",
		"test/profiles/git/render_test.go",
	}, "\n")
	if got := gitShow.Render(nameOnlyInv, engine.Execution{Stdout: nameOnlyStdout}); !strings.Contains(got, "internal/profiles/git/profile.go") || !strings.Contains(got, "test/profiles/git/render_test.go") {
		t.Fatalf("unexpected git-show name-only render output: %q", got)
	}

	if gitShow.StreamPreference != engine.StreamStdoutOnly || gitShow.StreamRender == nil {
		t.Fatalf("unexpected git-show stream metadata: %#v", gitShow)
	}
	stream := gitShow.StreamRender(statInv, engine.OutputBudget{MaxLines: 1})
	stream.ConsumeStdout([]byte(statStdout))
	if got := stream.Result(); !strings.Contains(got, "show abc123456789") {
		t.Fatalf("unexpected git-show stream output: %q", got)
	}
	provider, ok := stream.(interface {
		RecoveryInfo() (string, string, bool)
	})
	if !ok {
		t.Fatal("expected git-show stream reducer recovery provider")
	}
	if kind, summary, requireRawCapture := provider.RecoveryInfo(); kind != "full-output" || summary != "omitted git show details" || !requireRawCapture {
		t.Fatalf("unexpected git-show recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func assertGitStatusRender(t *testing.T, gitStatus engine.Profile) {
	t.Helper()
	if got := gitStatus.Render(engine.Invocation{}, engine.Execution{Stdout: "\x1b[31m## main\x1b[0m\nM  a\n"}); got == "" {
		t.Fatal("expected git-status render output")
	}
	if gitStatus.StreamPreference != engine.StreamStdoutOnly || gitStatus.StreamRender == nil || gitStatus.Budget.MaxLines < 3 {
		t.Fatalf("unexpected git-status stream metadata: %#v", gitStatus)
	}
	gitStatusStream := gitStatus.StreamRender(engine.Invocation{}, gitStatus.Budget)
	gitStatusStream.ConsumeStdout([]byte("## main\nM  a\n"))
	if got := gitStatusStream.Result(); got == "" || gitStatusStream.BytesParsed() == 0 {
		t.Fatalf("expected git-status stream output, got %q", got)
	}
}

func assertGitLogRender(t *testing.T, gitLog engine.Profile) {
	t.Helper()
	if got := gitLog.Render(engine.Invocation{}, engine.Execution{Stdout: "abc one\ndef two\n"}); got == "" {
		t.Fatal("expected git-log render output")
	}
	if gitLog.StreamPreference != engine.StreamStdoutOnly || gitLog.StreamRender == nil {
		t.Fatalf("unexpected git-log stream metadata: %#v", gitLog)
	}

	// An explicit user count must keep every requested commit visible, and
	// default-format output must be parsed per commit, not per line.
	countInv := engine.Classify(engine.Invocation{Command: []string{"git", "log", "-5"}})
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
		"commit d760fc9c245ba6a5138b60894eb8c52d2ca9500d",
		"Author: Arena <arena@example.com>",
		"Date:   Thu Jul 2 18:11:09 2026 -0400",
		"",
		"    chore: arena revision 78",
		"",
		"commit b57e68937e116eedbb007ecff25b4991774ebfff",
		"Author: Arena <arena@example.com>",
		"Date:   Thu Jul 2 18:11:09 2026 -0400",
		"",
		"    chore: arena revision 77",
		"",
	}, "\n")
	got := gitLog.Render(countInv, engine.Execution{Stdout: fullFormat})
	assertRenderContainsAll(t, got,
		"5 commits",
		"0472258 fix: correct arena fixture failures",
		"64c323b chore: arena revision 80",
		"6793330 chore: arena revision 79",
		"d760fc9 chore: arena revision 78",
		"b57e689 chore: arena revision 77",
	)
	if strings.Contains(got, "more commits") {
		t.Fatalf("expected no omitted commits for explicit count, got %q", got)
	}
	stream := gitLog.StreamRender(countInv, gitLog.Budget)
	stream.ConsumeStdout([]byte(fullFormat))
	if streamed := stream.Result(); streamed != got {
		t.Fatalf("expected git-log stream render to match render, got %q vs %q", streamed, got)
	}

	// Without an explicit count the compact two-entry preview is preserved.
	plainInv := engine.Classify(engine.Invocation{Command: []string{"git", "log"}, Display: []string{"git", "log"}})
	compact := gitLog.Render(plainInv, engine.Execution{Stdout: "a1 one\na2 two\na3 three\na4 four\n"})
	assertRenderContainsAll(t, compact, "4 commits", "a1 one", "a2 two", "... +2 more commits")
}

func assertGitDiffRender(t *testing.T, gitDiff engine.Profile) {
	t.Helper()
	if got := gitDiff.Render(engine.Invocation{}, engine.Execution{Stdout: "diff --git a/a b/a\n a | 1 +\n"}); got == "" {
		t.Fatal("expected git-diff render output")
	}
	if got := gitDiff.Render(engine.Invocation{}, engine.Execution{Stdout: "diff --git a/a.go b/a.go\n@@ -1 +1 @@ func demo() {\n+foo\n-bar\n"}); got == "" || got == "no diff" {
		t.Fatalf("expected git-diff patch render output, got %q", got)
	}
	largeStat := strings.Join([]string{
		"a.txt | 1 +",
		"b.txt | 8 ++++++--",
		"c.txt | 3 ++-",
		"d.txt | 12 +++++++++---",
		"e.txt | 2 +-",
		"f.txt | 20 ++++++++++++++------",
		"g.txt | 5 +++--",
		"h.txt | 7 +++++--",
		"i.txt | 4 ++--",
		"9 files changed, 62 insertions(+), 14 deletions(-)",
	}, "\n")
	got := gitDiff.Render(engine.Invocation{}, engine.Execution{Stdout: largeStat})
	assertRenderContainsAll(t, got, "files=9 +62 -14", "f.txt | 20", "d.txt | 12", "b.txt | 8", "... +5 more files")
	aggressive := gitDiff.Render(engine.Invocation{ReasoningBudgetMode: "aggressive"}, engine.Execution{Stdout: largeStat})
	if strings.Contains(aggressive, "... +5 more files") || !strings.Contains(aggressive, "... +7 more files") {
		t.Fatalf("expected aggressive git-diff render to keep fewer files, got %q", aggressive)
	}
	if gitDiff.StreamPreference != engine.StreamStdoutOnly || gitDiff.StreamRender == nil {
		t.Fatalf("unexpected git-diff stream metadata: %#v", gitDiff)
	}

	// A plain diff summarized from full patch data must keep filenames whole
	// (names longer than 8 characters must never be truncated).
	longName := strings.Join([]string{
		"diff --git a/calc/history_records.go b/calc/history_records.go",
		"index f2e139f..51f99f2 100644",
		"--- a/calc/history_records.go",
		"+++ b/calc/history_records.go",
		"@@ -20,3 +20,4 @@ func Describe(a int) string {",
		"+// pending local change",
	}, "\n")
	inv := engine.Classify(engine.Invocation{Command: []string{"git", "diff"}})
	smallDiffRender := gitDiff.Render(inv, engine.Execution{Stdout: longName})
	assertRenderContainsAll(t, smallDiffRender,
		"calc/history_records.go", "hunks=1", "+1 -0", "func Describe(a int) string {",
		// Small diffs keep their changed lines verbatim: the content of a
		// one-line change is the payload, not the churn stats.
		"+// pending local change")
	streamedSmall := gitDiff.StreamRender(inv, gitDiff.Budget)
	streamedSmall.ConsumeStdout([]byte(longName))
	if streamed := streamedSmall.Result(); streamed != smallDiffRender {
		t.Fatalf("expected git-diff stream render to match render, got %q vs %q", streamed, smallDiffRender)
	}

	// Conflicted files from combined diffs keep their full name and are marked.
	conflict := strings.Join([]string{
		"diff --cc conflicted.txt",
		"index 065e9d1,8209b71..0000000",
		"--- a/conflicted.txt",
		"+++ b/conflicted.txt",
		"@@@ -1,1 -1,1 +1,5 @@@",
		"++<<<<<<< HEAD",
		" +main change",
		"++=======",
		"+ side change",
		"++>>>>>>> side",
	}, "\n")
	conflictRender := gitDiff.Render(inv, engine.Execution{Stdout: conflict})
	assertRenderContainsAll(t, conflictRender, "conflicted.txt [conflict]", "hunks=1")
	if strings.Contains(conflictRender, "...icted.txt") {
		t.Fatalf("expected conflicted filename to stay whole, got %q", conflictRender)
	}
}

func assertRenderContainsAll(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %q", want, got)
		}
	}
}
