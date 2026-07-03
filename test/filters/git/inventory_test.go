package git_test

import (
	"fmt"
	"strings"
	"testing"

	gitfilter "github.com/devr-tools/szr/internal/filters/git"
)

// buildManyFileDiff synthesizes a patch-mode diff touching count files under
// a handful of directories, with one high-churn file carrying a needle
// constant, mirroring a wide refactor plus one real change.
func buildManyFileDiff(count int) string {
	var builder strings.Builder
	dirs := []string{"pkg/core", "pkg/api", "cmd/agentd", "internal/store"}
	for i := 0; i < count; i++ {
		path := fmt.Sprintf("%s/f%03d.go", dirs[i%len(dirs)], i)
		fmt.Fprintf(&builder, "diff --git a/%s b/%s\n", path, path)
		fmt.Fprintf(&builder, "index %07x..%07x 100644\n--- a/%s\n+++ b/%s\n", i+1, i+9, path, path)
		builder.WriteString("@@ -3,4 +3,4 @@ func handler() {\n")
		fmt.Fprintf(&builder, "-\tconst revision = %d\n", i)
		fmt.Fprintf(&builder, "+\tconst revision = %d\n", i+1)
		if i == 137 {
			builder.WriteString("@@ -20,3 +20,9 @@ func config() {\n")
			for j := 0; j < 6; j++ {
				fmt.Fprintf(&builder, "+\tconst keyRotationWindow%d = %d\n", j, j*30)
			}
		}
	}
	return builder.String()
}

// TestGitDiffManyFilesRendersFullInventory pins the inventory mode: when a
// diff touches far more files than the per-file summary budget, every
// filename stays discoverable (grouped by directory) instead of vanishing
// behind a "+N more files" tail, and the top-churn file keeps its full
// summary line.
func TestGitDiffManyFilesRendersFullInventory(t *testing.T) {
	t.Parallel()

	reducer := gitfilter.NewGitDiffReducer(9, 0)
	got := reducer.Reduce(buildManyFileDiff(300))

	assertContainsAll(t, got, "files=300")
	// The needle file has the highest churn and must be listed individually
	// with its hunk detail.
	assertContainsAll(t, got, "pkg/api/f137.go", "hunks=2", "func config() {")
	// Every other filename must be discoverable somewhere in the render.
	for _, name := range []string{"f000.go", "f042.go", "f138.go", "f299.go"} {
		if !strings.Contains(got, name) {
			t.Fatalf("expected filename %q to stay discoverable in inventory render:\n%s", name, got)
		}
	}
	// Directory groups carry counts so the shape of the change is scannable.
	if !strings.Contains(got, "pkg/core/ (") {
		t.Fatalf("expected directory-grouped inventory lines, got:\n%s", got)
	}
}

// TestGitDiffFewFilesKeepsPerFileSummaries pins that diffs within the
// summary budget keep the richer per-file summary lines instead of switching
// to the grouped inventory.
func TestGitDiffFewFilesKeepsPerFileSummaries(t *testing.T) {
	t.Parallel()

	reducer := gitfilter.NewGitDiffReducerWithOptions(gitfilter.GitDiffReducerOptions{MaxLines: 9, Aggressive: true})
	got := reducer.Reduce(buildManyFileDiff(4))
	assertContainsAll(t, got, "files=4", "pkg/core/f000.go", "internal/store/f003.go", "hunks=1")
	if strings.Contains(got, "more files") {
		t.Fatalf("expected no truncation tail for a small diff, got:\n%s", got)
	}
}
