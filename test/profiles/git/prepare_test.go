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

	gitStatus := testutil.FindProfile(t, list, "git-status")
	if !gitStatus.Match(engine.Invocation{Display: []string{"git", "status"}}) {
		t.Fatal("expected git status to match")
	}
	preparedStatus := gitStatus.Prepare(engine.Invocation{Command: []string{"git", "status"}})
	if preparedStatus[len(preparedStatus)-1] != "--branch" {
		t.Fatalf("unexpected prepared status: %#v", preparedStatus)
	}
	preparedStatus = gitStatus.Prepare(engine.Invocation{Command: []string{"git", "status", "--short"}})
	if len(preparedStatus) != 3 {
		t.Fatalf("expected passthrough status prepare: %#v", preparedStatus)
	}

	gitLog := testutil.FindProfile(t, list, "git-log")
	if !gitLog.Match(engine.Invocation{Display: []string{"git", "log"}}) {
		t.Fatal("expected git log to match")
	}
	if len(gitLog.Prepare(engine.Invocation{Command: []string{"git", "log"}})) <= 2 {
		t.Fatal("expected git-log prepare to add flags")
	}
	if len(gitLog.Prepare(engine.Invocation{Command: []string{"git", "log", "--format=%H"}})) != 3 {
		t.Fatal("expected git-log to preserve explicit format")
	}

	gitDiff := testutil.FindProfile(t, list, "git-diff")
	if !gitDiff.Match(engine.Invocation{Display: []string{"git", "diff"}}) {
		t.Fatal("expected git diff to match")
	}
	assertPreparedLengthAndArgs(t, "git diff standard", gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}, Advanced: advanced}), 6, []string{"--stat=96,24", "--compact-summary", "--no-color", "--no-ext-diff"})
	assertPreparedLengthAndArgs(t, "git diff explicit stat", gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff", "--stat"}, Advanced: advanced}), 5, []string{"--no-color", "--no-ext-diff"})
	assertPreparedLengthAndArgs(t, "git diff aggressive", gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}, ReasoningBudgetMode: "aggressive", Advanced: advanced}), 6, []string{"--stat=72,12"})
	assertPreparedLengthAndArgs(t, "git diff quiet", gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff", "--quiet"}, Advanced: advanced}), 3, nil)
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
