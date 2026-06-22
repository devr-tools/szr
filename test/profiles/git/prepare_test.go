package profiles_test

import (
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestGitProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)
	advanced := config.Default().Advanced

	gitAdd := testutil.FindProfile(t, list, "git-add")
	if !gitAdd.Match(engine.Classify(engine.Invocation{Display: []string{"git", "add", "README.md"}})) {
		t.Fatal("expected git add to match")
	}
	if !gitAdd.Match(engine.Classify(engine.Invocation{Display: []string{"git", "-C", "/tmp/worktree", "add", "."}})) {
		t.Fatal("expected git -C add to match")
	}

	gitCommit := testutil.FindProfile(t, list, "git-commit")
	if !gitCommit.Match(engine.Classify(engine.Invocation{Display: []string{"git", "commit", "-m", "msg"}})) {
		t.Fatal("expected git commit to match")
	}

	gitPush := testutil.FindProfile(t, list, "git-push")
	if !gitPush.Match(engine.Classify(engine.Invocation{Display: []string{"git", "push", "origin", "main"}})) {
		t.Fatal("expected git push to match")
	}

	gitPull := testutil.FindProfile(t, list, "git-pull")
	if !gitPull.Match(engine.Classify(engine.Invocation{Display: []string{"git", "pull", "--rebase"}})) {
		t.Fatal("expected git pull to match")
	}

	gitShow := testutil.FindProfile(t, list, "git-show")
	if !gitShow.Match(engine.Classify(engine.Invocation{Display: []string{"git", "show", "--stat", "HEAD"}})) {
		t.Fatal("expected git show --stat to match")
	}
	if !gitShow.Match(engine.Classify(engine.Invocation{Display: []string{"git", "-C", "/tmp/worktree", "show", "HEAD:internal/profiles/git/profile.go"}})) {
		t.Fatal("expected git -C show blob read to match")
	}
	assertPreparedLengthAndArgs(t, "git show stat", gitShow.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "show", "--stat", "HEAD"}})), 8, []string{"--no-color", "--no-ext-diff", "--no-patch", "--format=oneline"})
	assertPreparedLengthAndArgs(t, "git show name-only", gitShow.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "show", "--name-only", "HEAD"}})), 8, []string{"--no-color", "--no-ext-diff", "--no-patch", "--format=oneline"})
	assertPreparedLengthAndArgs(t, "git show blob", gitShow.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "show", "HEAD:internal/profiles/git/profile.go"}})), 5, []string{"--no-color", "--no-ext-diff"})
	assertPreparedLengthAndArgs(t, "git show explicit pretty", gitShow.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "show", "--stat", "--pretty=oneline", "HEAD"}})), 8, []string{"--no-color", "--no-ext-diff", "--no-patch"})
	assertPreparedLengthAndArgs(t, "git show path separator", gitShow.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "show", "--", "--stat"}})), 6, []string{"--no-color", "--no-ext-diff"})

	gitStatus := testutil.FindProfile(t, list, "git-status")
	if !gitStatus.Match(engine.Classify(engine.Invocation{Display: []string{"git", "status"}})) {
		t.Fatal("expected git status to match")
	}
	if !gitStatus.Match(engine.Classify(engine.Invocation{Display: []string{"git", "-C", "/tmp/worktree", "status"}})) {
		t.Fatal("expected git -C status to match")
	}
	preparedStatus := gitStatus.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "status"}}))
	if preparedStatus[len(preparedStatus)-1] != "--branch" {
		t.Fatalf("unexpected prepared status: %#v", preparedStatus)
	}
	preparedStatus = gitStatus.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "status", "--short"}}))
	if len(preparedStatus) != 3 {
		t.Fatalf("expected passthrough status prepare: %#v", preparedStatus)
	}

	gitLog := testutil.FindProfile(t, list, "git-log")
	if !gitLog.Match(engine.Classify(engine.Invocation{Display: []string{"git", "log"}})) {
		t.Fatal("expected git log to match")
	}
	if !gitLog.Match(engine.Classify(engine.Invocation{Display: []string{"git", "--no-pager", "-C", "/tmp/worktree", "log"}})) {
		t.Fatal("expected git global-option log to match")
	}
	if len(gitLog.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "log"}}))) <= 2 {
		t.Fatal("expected git-log prepare to add flags")
	}
	if len(gitLog.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "log", "--format=%H"}}))) != 3 {
		t.Fatal("expected git-log to preserve explicit format")
	}

	gitDiff := testutil.FindProfile(t, list, "git-diff")
	if !gitDiff.Match(engine.Classify(engine.Invocation{Display: []string{"git", "diff"}})) {
		t.Fatal("expected git diff to match")
	}
	if !gitDiff.Match(engine.Classify(engine.Invocation{Display: []string{"git", "-C", "/tmp/worktree", "diff"}})) {
		t.Fatal("expected git -C diff to match")
	}
	assertPreparedLengthAndArgs(t, "git diff standard", gitDiff.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "diff"}, Advanced: advanced})), 6, []string{"--stat=72,12", "--compact-summary", "--no-color", "--no-ext-diff"})
	assertPreparedLengthAndArgs(t, "git diff explicit stat", gitDiff.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "diff", "--stat"}, Advanced: advanced})), 5, []string{"--no-color", "--no-ext-diff"})
	assertPreparedLengthAndArgs(t, "git diff aggressive", gitDiff.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "diff"}, ReasoningBudgetMode: "aggressive", Advanced: advanced})), 6, []string{"--stat=56,8"})
	assertPreparedLengthAndArgs(t, "git diff quiet", gitDiff.Prepare(engine.Classify(engine.Invocation{Command: []string{"git", "diff", "--quiet"}, Advanced: advanced})), 3, nil)
}

func assertPreparedLengthAndArgs(t *testing.T, name string, got []string, wantLen int, wants []string) {
	t.Helper()
	if len(got) != wantLen {
		t.Fatalf("unexpected %s prepare: %#v", name, got)
	}
	for _, want := range wants {
		found := false
		for _, arg := range got {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %q in %s prepare: %#v", want, name, got)
		}
	}
}
