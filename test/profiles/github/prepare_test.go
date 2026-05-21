package profiles_test

import (
	"reflect"
	"testing"

	"szr/internal/engine"
	"szr/internal/profiles"
	"szr/test/testutil"
)

func TestGHProfilesPrepare(t *testing.T) {
	list := profiles.Builtins(6)

	ghPR := testutil.FindProfile(t, list, "gh-pr-view")
	if !ghPR.Match(engine.Invocation{Display: []string{"gh", "pr", "view"}}) {
		t.Fatal("expected gh pr view to match")
	}
	if !ghPR.Match(engine.Invocation{Display: []string{"gh", "-R", "owner/repo", "pr", "view", "42"}}) {
		t.Fatal("expected repo-scoped gh pr view to match")
	}
	if got := ghPR.Prepare(engine.Invocation{Command: []string{"gh", "pr", "view", "42"}}); !reflect.DeepEqual(got, []string{"gh", "pr", "view", "42", "--json", "number,title,state,isDraft,headRefName,baseRefName,reviewDecision,files"}) {
		t.Fatalf("unexpected gh pr prepare: %#v", got)
	}
	if got := ghPR.Prepare(engine.Invocation{Command: []string{"gh", "pr", "view", "--comments"}}); !reflect.DeepEqual(got, []string{"gh", "pr", "view", "--comments"}) {
		t.Fatalf("expected gh pr comments mode to be preserved: %#v", got)
	}

	ghRun := testutil.FindProfile(t, list, "gh-run-view")
	if !ghRun.Match(engine.Invocation{Display: []string{"gh", "--repo", "owner/repo", "run", "view", "123"}}) {
		t.Fatal("expected repo-scoped gh run view to match")
	}
	if got := ghRun.Prepare(engine.Invocation{Command: []string{"gh", "run", "view", "123"}}); !reflect.DeepEqual(got, []string{"gh", "run", "view", "123", "--json", "name,displayTitle,workflowName,status,conclusion,event,headBranch,jobs,url"}) {
		t.Fatalf("unexpected gh run prepare: %#v", got)
	}
	if got := ghRun.Prepare(engine.Invocation{Command: []string{"gh", "run", "view", "123", "--log"}}); !reflect.DeepEqual(got, []string{"gh", "run", "view", "123", "--log"}) {
		t.Fatalf("expected gh run log mode to be preserved: %#v", got)
	}

	ghRunLog := testutil.FindProfile(t, list, "gh-run-log")
	if !ghRunLog.Match(engine.Invocation{Display: []string{"gh", "run", "view", "123", "--log-failed"}}) {
		t.Fatal("expected gh run log profile to match explicit log mode")
	}
	if got := ghRunLog.Prepare(engine.Invocation{Command: []string{"gh", "run", "view", "123", "--log"}}); !reflect.DeepEqual(got, []string{"gh", "run", "view", "123", "--log"}) {
		t.Fatalf("expected gh run log profile to preserve raw log command: %#v", got)
	}

	ghChecks := testutil.FindProfile(t, list, "gh-pr-checks")
	if !ghChecks.Match(engine.Invocation{Display: []string{"gh", "pr", "checks", "42"}}) {
		t.Fatal("expected gh pr checks to match")
	}
	if got := ghChecks.Prepare(engine.Invocation{Command: []string{"gh", "pr", "checks", "42"}}); !reflect.DeepEqual(got, []string{"gh", "pr", "checks", "42"}) {
		t.Fatalf("expected gh pr checks prepare passthrough: %#v", got)
	}

	ghRunList := testutil.FindProfile(t, list, "gh-run-list")
	if !ghRunList.Match(engine.Invocation{Display: []string{"gh", "run", "list"}}) {
		t.Fatal("expected gh run list to match")
	}
	if got := ghRunList.Prepare(engine.Invocation{Command: []string{"gh", "run", "list", "--limit", "5"}}); !reflect.DeepEqual(got, []string{"gh", "run", "list", "--limit", "5"}) {
		t.Fatalf("expected gh run list prepare passthrough: %#v", got)
	}
}
