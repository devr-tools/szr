package git_test

import (
	"strings"
	"testing"

	gitfilter "github.com/devr-tools/szr/internal/filters/git"
)

func TestGitOpSummaries(t *testing.T) {
	t.Run("fetch", testGitOpFetch)
	t.Run("clone", testGitOpClone)
	t.Run("mergeConflict", testGitOpMergeConflict)
	t.Run("checkout", testGitOpCheckout)
	t.Run("stash", testGitOpStash)
	t.Run("failure", testGitOpFailure)
	t.Run("branch", testGitBranchSummary)
	t.Run("recovery", testGitOpRecovery)
}

func testGitOpFetch(t *testing.T) {
	t.Helper()
	input := strings.Join([]string{
		"remote: Enumerating objects: 120, done.",
		"remote: Counting objects: 100% (120/120), done.",
		"remote: Compressing objects: 100% (60/60), done.",
		"remote: Total 120 (delta 45), reused 100 (delta 40), pack-reused 0",
		"Receiving objects: 100% (120/120), 250.00 KiB | 2.00 MiB/s, done.",
		"Resolving deltas: 100% (45/45), done.",
		"From github.com:devr-tools/szr",
		"   dae313d..d3d7424  main       -> origin/main",
		" * [new branch]      feat-x     -> origin/feat-x",
	}, "\n")
	got := gitfilter.SummarizeGitOp("fetch", input, 8)
	assertContainsAll(t, got,
		"objects: 120 (250.00 KiB), deltas: 45",
		"From github.com:devr-tools/szr",
		"dae313d..d3d7424 main -> origin/main",
		"* [new branch] feat-x -> origin/feat-x",
	)
	for _, banned := range []string{"Enumerating", "Counting objects", "Compressing", "Receiving objects", "Resolving deltas", "remote:"} {
		if strings.Contains(got, banned) {
			t.Fatalf("expected fetch summary to drop %q, got %q", banned, got)
		}
	}
}

func testGitOpClone(t *testing.T) {
	t.Helper()
	input := strings.Join([]string{
		"Cloning into 'szr'...",
		"remote: Enumerating objects: 5230, done.",
		"remote: Counting objects: 100% (5230/5230), done.",
		"remote: Compressing objects: 100% (2100/2100), done.",
		"remote: Total 5230 (delta 3000), reused 5000 (delta 2900), pack-reused 0",
		"Receiving objects: 100% (5230/5230), 4.20 MiB | 8.40 MiB/s, done.",
		"Resolving deltas: 100% (3000/3000), done.",
	}, "\n")
	got := gitfilter.SummarizeGitOp("clone", input, 8)
	assertContainsAll(t, got, "Cloning into 'szr'...", "objects: 5230 (4.20 MiB), deltas: 3000")
	if lines := strings.Split(got, "\n"); len(lines) != 2 {
		t.Fatalf("expected clone summary to collapse to 2 lines, got %d: %q", len(lines), got)
	}
}

func testGitOpMergeConflict(t *testing.T) {
	t.Helper()
	conflictFiles := []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go"}
	lines := []string{}
	for _, file := range conflictFiles {
		lines = append(lines, "Auto-merging "+file)
		lines = append(lines, "CONFLICT (content): Merge conflict in "+file)
	}
	lines = append(lines, "Automatic merge failed; fix conflicts and then commit the result.")
	input := strings.Join(lines, "\n")
	for _, maxLines := range []int{6, 12} {
		got := gitfilter.SummarizeGitOp("merge", input, maxLines)
		for _, file := range conflictFiles {
			if !strings.Contains(got, "CONFLICT (content): Merge conflict in "+file) {
				t.Fatalf("expected every conflict path to survive at maxLines=%d, missing %s in %q", maxLines, file, got)
			}
		}
		if !strings.Contains(got, "Automatic merge failed") {
			t.Fatalf("expected merge failure line at maxLines=%d, got %q", maxLines, got)
		}
	}
	roomy := gitfilter.SummarizeGitOp("merge", input, 12)
	assertContainsAll(t, roomy, "Auto-merging: a.go, b.go, c.go (+5 more)")
}

func testGitOpCheckout(t *testing.T) {
	t.Helper()
	got := gitfilter.SummarizeGitOp("checkout", strings.Join([]string{
		"Updating files: 100% (1234/1234), done.",
		"Switched to branch 'main'",
		"Your branch is up to date with 'origin/main'.",
	}, "\n"), 8)
	assertContainsAll(t, got, "Switched to branch 'main'", "Your branch is up to date with 'origin/main'.")
	if strings.Contains(got, "Updating files") {
		t.Fatalf("expected checkout progress to be dropped, got %q", got)
	}
}

func testGitOpStash(t *testing.T) {
	t.Helper()
	got := gitfilter.SummarizeGitOp("stash", "Saved working directory and index state WIP on main: dae313d subject\n", 8)
	if got != "Saved working directory and index state WIP on main: dae313d subject" {
		t.Fatalf("unexpected stash summary: %q", got)
	}
	list := gitfilter.SummarizeGitOp("stash", "stash@{0}: WIP on main: dae313d one\nstash@{1}: WIP on main: 882784c two\n", 8)
	assertContainsAll(t, list, "stash@{0}: WIP on main: dae313d one", "stash@{1}: WIP on main: 882784c two")
}

func testGitOpFailure(t *testing.T) {
	t.Helper()
	input := strings.Join([]string{
		"Receiving objects:  42% (2000/5230)",
		"fatal: unable to access 'https://github.com/x/y.git/': Could not resolve host: github.com",
		"error: could not apply abc1234... subject",
		"hint: Resolve all conflicts manually, mark them as resolved with",
		"hint: \"git add/rm <conflicted_files>\", then run \"git rebase --continue\".",
	}, "\n")
	got := gitfilter.SummarizeGitOp("rebase", input, 4)
	assertContainsAll(t, got,
		"fatal: unable to access 'https://github.com/x/y.git/': Could not resolve host: github.com",
		"error: could not apply abc1234... subject",
		"hint: Resolve all conflicts manually, mark them as resolved with",
	)
	if strings.Contains(got, "42%") {
		t.Fatalf("expected progress percentages to be dropped on failure, got %q", got)
	}
}

func testGitBranchSummary(t *testing.T) {
	t.Helper()
	short := "* main\n  improvments\n"
	if got := gitfilter.SummarizeGitBranches(short, 8); got != "* main\n  improvments" {
		t.Fatalf("expected short branch listing passthrough, got %q", got)
	}

	lines := []string{"* main"}
	for _, name := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"} {
		lines = append(lines, "  feature/"+name)
	}
	got := gitfilter.SummarizeGitBranches(strings.Join(lines, "\n"), 4)
	assertContainsAll(t, got, "11 branches (current: main)", "main", "feature/... (10)")
	if strings.Contains(got, "feature/one") {
		t.Fatalf("expected long branch listing to be grouped, got %q", got)
	}
}

func testGitOpRecovery(t *testing.T) {
	t.Helper()
	input := strings.Join([]string{
		"remote: Counting objects: 100% (10/10), done.",
		"Receiving objects: 100% (10/10), 1.00 KiB | 1.00 MiB/s, done.",
		"From github.com:devr-tools/szr",
		"   aaa1111..bbb2222  main -> origin/main",
	}, "\n")
	kind, summary, requireRawCapture := gitfilter.GitOpRecoveryInfo("fetch", input, 8)
	if kind != "full-output" || summary != "omitted git fetch progress and detail lines" || !requireRawCapture {
		t.Fatalf("unexpected git op recovery info: kind=%q summary=%q raw=%v", kind, summary, requireRawCapture)
	}
	if kind, _, _ := gitfilter.GitOpRecoveryInfo("fetch", "Already up to date.\n", 8); kind != "" {
		t.Fatalf("expected no recovery for fully-kept output, got kind=%q", kind)
	}
}
