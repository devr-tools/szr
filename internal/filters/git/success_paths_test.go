package git

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
)

func TestGitSuccessPathSummary(t *testing.T) {
	addInv := engine.Classify(engine.Invocation{Command: []string{"git", "add", "internal/profiles/git/profile.go", "test/profiles/git/render_test.go"}})
	if got := SummarizeGitAdd(addInv, ""); got != "staged internal/... (1), test/... (1)" {
		t.Fatalf("unexpected git add summary: %q", got)
	}
	addAllInv := engine.Classify(engine.Invocation{Command: []string{"git", "add", "-A"}})
	if got := SummarizeGitAdd(addAllInv, ""); got != "staged all changes" {
		t.Fatalf("unexpected git add all summary: %q", got)
	}
	displayOnlyAdd := engine.Invocation{Display: []string{"git", "-C", "/tmp/worktree", "add", "--", "README.md"}}
	if got := SummarizeGitAdd(displayOnlyAdd, ""); got != "staged README.md" {
		t.Fatalf("unexpected git add display fallback summary: %q", got)
	}
	commitInv := engine.Classify(engine.Invocation{Command: []string{"git", "commit", "-m", "tighten reducer"}})
	if got := SummarizeGitCommit(commitInv, "[main abc1234] tighten reducer\n 2 files changed, 7 insertions(+), 1 deletion(-)\n"); got != "committed abc1234 tighten reducer files=2 +7 -1" {
		t.Fatalf("unexpected git commit summary: %q", got)
	}
	commitFallbackInv := engine.Classify(engine.Invocation{Command: []string{"git", "commit", "--message=subject from args\nbody line"}})
	if got := SummarizeGitCommit(commitFallbackInv, "[main abc1234] \n 1 file changed, 2 insertions(+)\n"); got != "committed abc1234 subject from args files=1 +2 -0" {
		t.Fatalf("unexpected git commit fallback-subject summary: %q", got)
	}
	if got := SummarizeGitPush("To github.com:devr-tools/szr.git\n   abc1234..def5678  main -> main\n"); got != "pushed main abc1234..def5678" {
		t.Fatalf("unexpected git push summary: %q", got)
	}
	if got := SummarizeGitPush("Everything up-to-date\n"); got != "push up-to-date" {
		t.Fatalf("unexpected git up-to-date push summary: %q", got)
	}
	if got := SummarizeGitPush("   abc1234..def5678  main -> main\n * [new tag]         v1.0.0 -> v1.0.0\n"); got != "pushed 2 refs" {
		t.Fatalf("unexpected multi-ref git push summary: %q", got)
	}
	if got := SummarizeGitPush(" * [new branch]      feature/demo -> feature/demo\n"); got != "pushed new branch feature/demo" {
		t.Fatalf("unexpected git new-branch push summary: %q", got)
	}
	if got := SummarizeGitPush(" + abc1234..def5678 main -> main (forced update)\n"); got != "force-pushed main" {
		t.Fatalf("unexpected git forced push summary: %q", got)
	}
	if got := SummarizeGitPull("From github.com:devr-tools/szr\nAlready up to date.\n"); got != "pull up-to-date" {
		t.Fatalf("unexpected git pull up-to-date summary: %q", got)
	}
	if got := SummarizeGitPull("Updating abc1234..def5678\nFast-forward\n 1 file changed, 3 insertions(+)\n"); got != "pulled abc1234..def5678 fast-forward files=1 +3 -0" {
		t.Fatalf("unexpected git pull summary: %q", got)
	}
}

func TestGitSuccessReducers(t *testing.T) {
	pushReducer := NewGitSuccessPathReducer("push", engine.Invocation{}, 6, 0)
	pushReducer.ConsumeStderr([]byte("To github.com:devr-tools/szr.git\n"))
	pushReducer.ConsumeStderr([]byte("   abc1234..def5678  main -> main\n"))
	if got := pushReducer.Result(); got != "pushed main abc1234..def5678" {
		t.Fatalf("unexpected git push reducer summary: %q", got)
	}
	if pushReducer.FallbackUsed() {
		t.Fatal("expected recognized git push success path")
	}
	if kind, summary, requireRawCapture := pushReducer.RecoveryInfo(); kind != "full-output" || summary != "omitted git push success details" || !requireRawCapture {
		t.Fatalf("unexpected git push recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
	if pushReducer.BytesParsed() == 0 {
		t.Fatal("expected git push reducer to track parsed bytes")
	}
	if preview := pushReducer.Preview(); preview != "pushed main abc1234..def5678" {
		t.Fatalf("unexpected git push reducer preview: %q", preview)
	}

	commitReducer := NewGitSuccessPathReducer("commit", engine.Invocation{}, 6, 0)
	commitReducer.ConsumeStdout([]byte("[main abc1234] tighten reducer\n 2 files changed, 7 insertions(+), 1 deletion(-)\n"))
	if got := commitReducer.Result(); got != "committed abc1234 tighten reducer files=2 +7 -1" {
		t.Fatalf("unexpected git commit reducer summary: %q", got)
	}

	fallbackReducer := NewGitSuccessPathReducer("pull", engine.Invocation{}, 6, 0)
	fallbackReducer.ConsumeStdout([]byte("CONFLICT (content): Merge conflict in internal/profiles/git/profile.go\n"))
	if !fallbackReducer.FallbackUsed() {
		t.Fatal("expected unrecognized git pull output to fall back")
	}
	if got := fallbackReducer.Result(); !strings.Contains(got, "CONFLICT (content): Merge conflict") {
		t.Fatalf("expected git pull fallback to preserve raw detail, got %q", got)
	}
	if preview := fallbackReducer.Preview(); !strings.Contains(preview, "CONFLICT (content): Merge conflict") {
		t.Fatalf("expected git pull fallback preview to preserve raw detail, got %q", preview)
	}
}
