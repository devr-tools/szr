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

	gitStatus := testutil.FindProfile(t, list, "git-status")
	assertGitStatusRender(t, gitStatus)

	gitLog := testutil.FindProfile(t, list, "git-log")
	assertGitLogRender(t, gitLog)

	gitDiff := testutil.FindProfile(t, list, "git-diff")
	assertGitDiffRender(t, gitDiff)
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
	assertRenderContainsAll(t, got, "files=9 +62 -14", "f.txt | 20", "d.txt | 12", "b.txt | 8", "... +4 more files")
	aggressive := gitDiff.Render(engine.Invocation{ReasoningBudgetMode: "aggressive"}, engine.Execution{Stdout: largeStat})
	if strings.Contains(aggressive, "... +4 more files") || !strings.Contains(aggressive, "... +6 more files") {
		t.Fatalf("expected aggressive git-diff render to keep fewer files, got %q", aggressive)
	}
	if gitDiff.StreamPreference != engine.StreamStdoutOnly || gitDiff.StreamRender == nil {
		t.Fatalf("unexpected git-diff stream metadata: %#v", gitDiff)
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
