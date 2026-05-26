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
	if len(gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}, Advanced: advanced})) != 6 {
		t.Fatal("expected git-diff to add summary flags")
	}
	if prepared := gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}, Advanced: advanced}); len(prepared) != 6 || prepared[2] != "--stat=96,24" || prepared[3] != "--compact-summary" || prepared[4] != "--no-color" || prepared[5] != "--no-ext-diff" {
		t.Fatalf("unexpected standard git-diff prepare: %#v", prepared)
	}
	if prepared := gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff", "--stat"}, Advanced: advanced}); len(prepared) != 5 || prepared[3] != "--no-color" || prepared[4] != "--no-ext-diff" {
		t.Fatalf("expected git-diff to preserve explicit stat mode and add only noise flags: %#v", prepared)
	}
	if prepared := gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff"}, ReasoningBudgetMode: "aggressive", Advanced: advanced}); len(prepared) != 6 || prepared[2] != "--stat=72,12" {
		t.Fatalf("unexpected aggressive git-diff prepare: %#v", prepared)
	}
	if prepared := gitDiff.Prepare(engine.Invocation{Command: []string{"git", "diff", "--quiet"}, Advanced: advanced}); len(prepared) != 3 {
		t.Fatalf("expected quiet git-diff to pass through: %#v", prepared)
	}
}
