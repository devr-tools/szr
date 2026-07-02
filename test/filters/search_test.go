package filters_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
)

func TestRipgrepAndTreeHelpers(t *testing.T) {
	if got := filters.GroupRipgrep("garbage\n", 2); got != "no matches" {
		t.Fatalf("unexpected no matches: %q", got)
	}
	grouped := filters.GroupRipgrep(strings.Join([]string{
		"one.go:1:first",
		"one.go:2:second",
		"one.go:3:third",
		"one.go:4:fourth",
		"two.go:9:two",
		"three.go:7:three",
	}, "\n"), 2)
	for _, want := range []string{"6 matches across 3 files", "one.go:1: first (4 matches)", "... +1 more files"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in grouped output:\n%s", want, grouped)
		}
	}

	if !filters.ShouldSkipTreePart(".") || !filters.ShouldSkipTreePart("") || filters.ShouldSkipTreePart("dir") {
		t.Fatal("unexpected tree part skip behavior")
	}
	parts := filters.SplitTreeParts("/root", "/root/file.go", func(root, path string) (string, error) {
		return "dir/file.go", nil
	})
	if strings.Join(parts, ",") != "dir,file.go" {
		t.Fatalf("unexpected split tree parts: %#v", parts)
	}
	parts = filters.SplitTreeParts("/root", "/root/file.go", func(root, path string) (string, error) {
		return "", errors.New("boom")
	})
	if parts != nil {
		t.Fatalf("expected nil parts on split error: %#v", parts)
	}

	tree := filters.BuildTree([]string{
		"/root",
		"/root/a.go",
		"/root/dir/b.go",
		"/root/dir/sub/c.go",
		"/root/dir/sub/deeper/d.go",
		"/root//double//slashes.go",
		"/other/outside.go",
	}, "/root")
	for _, want := range []string{"root", "a.go", "dir", "sub"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("expected %q in tree:\n%s", want, tree)
		}
	}
}

func TestBuildTreeCollapsesSingleChildChains(t *testing.T) {
	tree := filters.BuildTree([]string{
		"/root/a/b/c/d/leaf.go",
	}, "/root")
	lines := strings.Split(tree, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected collapsed chain to render 3 lines, got %d:\n%s", len(lines), tree)
	}
	if lines[1] != "  a/b/c/d/" {
		t.Fatalf("expected collapsed chain line %q, got %q", "  a/b/c/d/", lines[1])
	}
	if strings.TrimSpace(lines[2]) != "leaf.go" {
		t.Fatalf("expected leaf line, got %q", lines[2])
	}
}

func TestBuildTreeCapsLines(t *testing.T) {
	paths := make([]string, 0, 150)
	for i := 0; i < 150; i++ {
		paths = append(paths, fmt.Sprintf("/root/file-%03d.txt", i))
	}
	tree := filters.BuildTree(paths, "/root")
	lines := strings.Split(tree, "\n")
	if len(lines) != 101 {
		t.Fatalf("expected capped tree of 101 lines, got %d", len(lines))
	}
	if lines[len(lines)-1] != "... +51 more" {
		t.Fatalf("expected overflow marker, got %q", lines[len(lines)-1])
	}
}

func TestGroupRipgrepClipsRepresentativeMatch(t *testing.T) {
	long := strings.Repeat("x", 200)
	grouped := filters.GroupRipgrep("big.go:7:"+long+"\n", 2)
	if !strings.Contains(grouped, "big.go:7: "+strings.Repeat("x", 120)+"... (1 matches)") {
		t.Fatalf("expected clipped representative match line, got:\n%s", grouped)
	}
}

func TestFindSummaries(t *testing.T) {
	if got := filters.SummarizeFindPaths(nil, 4); got != "no matches" {
		t.Fatalf("unexpected empty find summary: %q", got)
	}
	got := filters.SummarizeFindPaths([]string{"b.py", "a.py"}, 4)
	for _, want := range []string{"2 matches | ext: .py (2)", "examples: a.py, b.py"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in find summary:\n%s", want, got)
		}
	}
	truncated := filters.SummarizeFindPaths([]string{"a.py", "b.py", "c.py", "d.py"}, 3)
	if !strings.Contains(truncated, "4 matches | ext: .py (4)") {
		t.Fatalf("expected truncated find summary, got %q", truncated)
	}
	suppressed := filters.SummarizeFindPaths([]string{"node_modules/a.js", "src/a.py"}, 4)
	if !strings.Contains(suppressed, "suppressed noisy paths") {
		t.Fatalf("expected suppressed-path note, got %q", suppressed)
	}
	reducerOnlySuppressed := filters.SummarizeFindPaths([]string{".venv/bin/python", "tmp/cache.txt", "src/a.py"}, 4)
	for _, want := range []string{".venv", "tmp"} {
		if !strings.Contains(reducerOnlySuppressed, want) {
			t.Fatalf("expected reducer-only noise bucket %q in find summary:\n%s", want, reducerOnlySuppressed)
		}
	}
	ideSuppressed := filters.SummarizeFindPaths([]string{
		".idea/workspace.xml",
		".vscode/settings.json",
		".pytest_cache/v/cache",
		".tox/py311/bin/python",
		".terraform/modules/main.tf",
		".nyc_output/out.json",
		"Pods/Alamofire/Source/AF.swift",
		"DerivedData/Build/app.o",
		"src/a.py",
	}, 4)
	if !strings.Contains(ideSuppressed, "1 matches") || !strings.Contains(ideSuppressed, "suppressed noisy paths: 8") {
		t.Fatalf("expected new noise dirs suppressed, got %q", ideSuppressed)
	}
	bundleSuppressed := filters.SummarizeFindPaths([]string{
		"dist2/app.bundle.js",
		"static/app.min.mjs",
		"static/app.mjs.map",
		"static/app.cjs.map",
		"src/a.js",
	}, 4)
	for _, want := range []string{"1 matches", "minified assets", "source maps"} {
		if !strings.Contains(bundleSuppressed, want) {
			t.Fatalf("expected %q in bundle suppression summary:\n%s", want, bundleSuppressed)
		}
	}
	grouped := filters.SummarizeFindPathsGrouped([]string{"cmd/a.go", "cmd/b.go", "internal/c.go"}, 4)
	for _, want := range []string{"3F 2D", "cmd/ a.go b.go", "internal/ c.go"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in grouped find summary:\n%s", want, grouped)
		}
	}
}

func TestSummarizeRipgrep(t *testing.T) {
	grouped := filters.SummarizeRipgrep(strings.Join([]string{
		"one.go:1:first",
		"one.go:2:second",
		"two.go:9:two",
	}, "\n"), 4, 6)
	for _, want := range []string{"one.go:1: first (2 matches)", "two.go:9: two (1 matches)"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in ripgrep summary:\n%s", want, grouped)
		}
	}

	fallback := filters.SummarizeRipgrep("rg: ./vendor: Permission denied (os error 13)\n", 4, 4)
	if !strings.Contains(fallback, "Permission denied") {
		t.Fatalf("expected ripgrep error fallback, got %q", fallback)
	}
}

func TestStreamingSearchReducers(t *testing.T) {
	rg := filters.NewRipgrepReducer(2, 8)
	rg.ConsumeStdout([]byte("a.go:1:first\na.go:2:second\n"))
	rg.ConsumeStdout([]byte("b.go:9:third\n"))
	rg.ConsumeStdout([]byte("c.go:4:fourth\n"))
	rg.ConsumeStdout([]byte("node_modules/pkg/c.go:1:ignored\n"))
	if !rg.Done() {
		t.Fatal("expected ripgrep reducer to report done after filling visible groups")
	}
	if got := rg.Result(); !strings.Contains(got, "suppressed noisy paths") {
		t.Fatalf("expected ripgrep reducer suppression note, got %q", got)
	}
	if preview, result := rg.Preview(), rg.Result(); preview != result {
		t.Fatalf("expected stable ripgrep preview/result, preview=%q result=%q", preview, result)
	}
	if kind, summary, requireRawCapture := rg.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 1 additional files" || !requireRawCapture {
		t.Fatalf("unexpected ripgrep recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	find := filters.NewFindReducer(4)
	find.ConsumeStdout([]byte("/tmp/a.py\n/tmp/b.py\n/tmp/c.py\n/tmp/d.py\n/tmp/e.py\n"))
	if !find.Done() {
		t.Fatal("expected find reducer to report done after filling sample budget")
	}
	if got := find.Result(); !strings.Contains(got, "examples: /tmp/a.py") {
		t.Fatalf("expected find reducer compact examples, got %q", got)
	}
	if preview, result := find.Preview(), find.Result(); preview != result {
		t.Fatalf("expected stable find preview/result, preview=%q result=%q", preview, result)
	}
	if kind, summary, requireRawCapture := find.RecoveryInfo(); kind != filters.RecoveryKindFullOutput || summary != "omitted 2 additional matches" || !requireRawCapture {
		t.Fatalf("unexpected find recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestStreamingRipgrepReducerMatchRecovery(t *testing.T) {
	rg := filters.NewRipgrepReducer(2, 8)
	rg.ConsumeStdout([]byte("a.go:1:first\na.go:2:second\na.go:3:third\na.go:4:fourth\n"))
	if got := rg.Result(); !strings.Contains(got, "a.go:1: first (4 matches)") {
		t.Fatalf("expected ripgrep reducer count summary, got %q", got)
	}
	if kind, summary, requireRawCapture := rg.RecoveryInfo(); kind != "" || summary != "" || requireRawCapture {
		t.Fatalf("unexpected ripgrep match recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestStreamingSearchReducersSuppressReducerOnlyNoise(t *testing.T) {
	rg := filters.NewRipgrepReducer(2, 6)
	rg.ConsumeStdout([]byte(".venv/lib/site.py:1:ignored\n"))
	rg.ConsumeStdout([]byte("src/generated.js.map:2:ignored\n"))
	rg.ConsumeStdout([]byte("src/app.go:3:kept\n"))
	got := rg.Result()
	for _, want := range []string{"src/app.go:3: kept (1 matches)", ".venv", "source maps"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in ripgrep reducer output:\n%s", want, got)
		}
	}

	find := filters.NewFindReducer(4)
	find.ConsumeStdout([]byte(".venv/bin/python\ntmp/build.log\nsrc/app.go\n"))
	got = find.Result()
	for _, want := range []string{"1 matches | ext: .go (1)", "examples: src/app.go"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in find reducer output:\n%s", want, got)
		}
	}
}
