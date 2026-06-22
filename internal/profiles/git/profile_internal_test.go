package git

import "testing"

func TestExtractGitShowPaths(t *testing.T) {
	paths, ok := extractGitShowPaths([]string{"M\tinternal/profiles/git/profile.go", "A\ttest/profiles/git/render_test.go"})
	if !ok {
		t.Fatal("expected status-style git show paths to be extracted")
	}
	if len(paths) != 2 || paths[0] != "internal/profiles/git/profile.go" || paths[1] != "test/profiles/git/render_test.go" {
		t.Fatalf("unexpected extracted paths: %#v", paths)
	}

	if paths, ok := extractGitShowPaths([]string{"internal/profiles/git/profile.go", "test/profiles/git/render_test.go"}); !ok || len(paths) != 2 {
		t.Fatalf("expected plain path list to be preserved, got ok=%v paths=%#v", ok, paths)
	}

	if paths, ok := extractGitShowPaths([]string{"internal/profiles/git/profile.go | 12 +++++++++---"}); ok || paths != nil {
		t.Fatalf("expected diff stat line to reject path extraction, got ok=%v paths=%#v", ok, paths)
	}
}

func TestIsGitShowBodyLine(t *testing.T) {
	for _, line := range []string{
		"diff --git a/a.go b/a.go",
		"@@ -1 +1 @@ func demo() {",
		" 1 file changed, 2 insertions(+)",
		" a.go | 2 +-",
		"M\tinternal/profiles/git/profile.go",
		"plain content",
	} {
		if !isGitShowBodyLine(line) {
			t.Fatalf("expected body line: %q", line)
		}
	}
	for _, line := range []string{
		"Author: Dev",
		"Date: today",
		"commit abc1234",
		"    indented commit message",
	} {
		if isGitShowBodyLine(line) {
			t.Fatalf("expected non-body line: %q", line)
		}
	}
}

func TestGitShowRequestedFlags(t *testing.T) {
	if !gitShowSummaryRequested([]string{"git", "show", "--stat", "HEAD"}) {
		t.Fatal("expected --stat to request summary mode")
	}
	if !gitShowSummaryRequested([]string{"git", "show", "--name-only", "HEAD"}) {
		t.Fatal("expected --name-only to request summary mode")
	}
	if gitShowSummaryRequested([]string{"git", "show", "--", "--stat"}) {
		t.Fatal("expected args after -- to be ignored for summary detection")
	}

	if !gitShowPrettyRequested([]string{"git", "show", "--pretty=oneline", "HEAD"}) {
		t.Fatal("expected --pretty=oneline to request pretty mode")
	}
	if !gitShowPrettyRequested([]string{"git", "show", "--format=%H", "HEAD"}) {
		t.Fatal("expected --format to request pretty mode")
	}
	if gitShowPrettyRequested([]string{"git", "show", "--", "--pretty=oneline"}) {
		t.Fatal("expected args after -- to be ignored for pretty detection")
	}
}
