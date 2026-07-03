package profiles_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

// TestItemListProfile pins the gh/glab list profile: matching for both CLIs
// (glab's `mr` head maps to `pr`), JSON-mode prepare per CLI, and a render
// that keeps every title as a one-line item.
func TestItemListProfile(t *testing.T) {
	list := profiles.Builtins(6)
	itemList := testutil.FindProfile(t, list, "gh-item-list")

	for _, display := range [][]string{
		{"gh", "pr", "list"},
		{"gh", "issue", "list"},
		{"glab", "mr", "list"},
		{"glab", "issue", "list"},
		{"glab", "-R", "group/project", "mr", "list"},
	} {
		if !itemList.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected gh-item-list to match %v", display)
		}
	}
	for _, display := range [][]string{
		{"glab", "mr", "view", "42"},
		{"gh", "pr", "view", "42"},
		{"glab", "ci", "list"},
	} {
		if itemList.Match(engine.Invocation{Display: display}) {
			t.Fatalf("expected gh-item-list not to match %v", display)
		}
	}

	if got := itemList.Prepare(engine.Invocation{Command: []string{"glab", "mr", "list"}}); !reflect.DeepEqual(got, []string{"glab", "mr", "list", "--output", "json"}) {
		t.Fatalf("unexpected glab mr list prepare: %#v", got)
	}
	if got := itemList.Prepare(engine.Invocation{Command: []string{"glab", "mr", "list", "-F", "json"}}); !reflect.DeepEqual(got, []string{"glab", "mr", "list", "-F", "json"}) {
		t.Fatalf("expected explicit glab formatter to be preserved: %#v", got)
	}
	if got := itemList.Prepare(engine.Invocation{Command: []string{"gh", "pr", "list"}}); !reflect.DeepEqual(got, []string{"gh", "pr", "list", "--json", "number,title,state,isDraft,headRefName,baseRefName"}) {
		t.Fatalf("unexpected gh pr list prepare: %#v", got)
	}
	if got := itemList.Prepare(engine.Invocation{Command: []string{"gh", "issue", "list"}}); !reflect.DeepEqual(got, []string{"gh", "issue", "list", "--json", "number,title,state"}) {
		t.Fatalf("unexpected gh issue list prepare: %#v", got)
	}

	rendered := itemList.Render(engine.Invocation{}, engine.Execution{
		Stdout: `[{"iid":91,"title":"feat(export): stream CSV rows","state":"opened","source_branch":"feat/csv-stream","target_branch":"main"}]`,
	})
	for _, want := range []string{"items: 1", "#91 opened feat(export): stream CSV rows feat/csv-stream->main"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in item list render:\n%s", want, rendered)
		}
	}
}

// TestGlabMRViewMapsToPRProfile pins the glab head mapping on the existing
// PR view profile: `glab mr view` matches, prepare uses glab's JSON flag
// instead of gh's field list, and the render accepts GitLab's JSON shape.
func TestGlabMRViewMapsToPRProfile(t *testing.T) {
	list := profiles.Builtins(6)
	ghPR := testutil.FindProfile(t, list, "gh-pr-view")

	if !ghPR.Match(engine.Invocation{Display: []string{"glab", "mr", "view", "91"}}) {
		t.Fatal("expected glab mr view to match the PR view profile")
	}
	if got := ghPR.Prepare(engine.Invocation{Command: []string{"glab", "mr", "view", "91"}}); !reflect.DeepEqual(got, []string{"glab", "mr", "view", "91", "--output", "json"}) {
		t.Fatalf("unexpected glab mr view prepare: %#v", got)
	}
	if got := ghPR.Prepare(engine.Invocation{Command: []string{"glab", "mr", "view", "91", "--output", "json"}}); !reflect.DeepEqual(got, []string{"glab", "mr", "view", "91", "--output", "json"}) {
		t.Fatalf("expected explicit glab output flag to be preserved: %#v", got)
	}

	rendered := ghPR.Render(engine.Invocation{}, engine.Execution{
		Stdout: `{"iid":91,"title":"feat(export): stream CSV rows","state":"opened","draft":true,"source_branch":"feat/csv-stream","target_branch":"main"}`,
	})
	for _, want := range []string{"#91 OPENED feat(export): stream CSV rows", "[draft]", "feat/csv-stream->main"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in glab mr view render:\n%s", want, rendered)
		}
	}
}
