package test

import (
	"errors"
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestFilterHelpers(t *testing.T) {
	if got := filters.CompactLines("a\nb\nc", 2); got != "a\nb\n... +1 more lines" {
		t.Fatalf("unexpected compact lines: %q", got)
	}
	if got := filters.CompactLines("a\nb", 5); got != "a\nb" {
		t.Fatalf("unexpected compact passthrough: %q", got)
	}

	deduped := filters.DedupeLines("a\na\nb\nc\nc\n", 2)
	if deduped != "a (x2)\nb\n... +1 more unique lines" {
		t.Fatalf("unexpected dedupe: %q", deduped)
	}

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
	for _, want := range []string{"one.go (4 matches)", "  ... +1 more", "... +1 more files"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in grouped output:\n%s", want, grouped)
		}
	}

	if got := filters.SummarizeGitStatus(""); got != "clean" {
		t.Fatalf("unexpected clean status: %q", got)
	}
	status := filters.SummarizeGitStatus("## main\nM  a\n A b\n?? c\nx\n")
	if !strings.Contains(status, "staged=1 unstaged=1 untracked=1") {
		t.Fatalf("unexpected status summary: %q", status)
	}
	status = filters.SummarizeGitStatus("M  a\nM  b\nM  c\nM  d\nM  e\nM  f\nM  g\n")
	if strings.Count(status, "\n  ") != 6 {
		t.Fatalf("expected file preview to cap at 6 entries: %q", status)
	}

	if got := filters.SummarizeGitLog(""); got != "no commits" {
		t.Fatalf("unexpected empty git log: %q", got)
	}
	log := filters.SummarizeGitLog(strings.Repeat("hash subject\n", 12))
	if !strings.HasPrefix(log, "12 commits\n") {
		t.Fatalf("unexpected git log summary: %q", log)
	}

	if got := filters.SummarizeGitDiff(""); got != "no diff" {
		t.Fatalf("unexpected empty diff: %q", got)
	}
	diffStat := filters.SummarizeGitDiff(strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"+++ b/a.go",
		"--- a/a.go",
		"+foo",
		"-bar",
		" a.go | 2 +-",
		" 1 file changed, 1 insertion(+), 1 deletion(-)",
	}, "\n"))
	if !strings.Contains(diffStat, "files=1 +1 -1") || !strings.Contains(diffStat, "1 file changed") {
		t.Fatalf("unexpected diff stat: %q", diffStat)
	}
	diffFallback := filters.SummarizeGitDiff("diff --git a/a b/a\n+foo\n-bar")
	if !strings.Contains(diffFallback, "... +") && !strings.Contains(diffFallback, "diff --git") {
		t.Fatalf("unexpected diff fallback: %q", diffFallback)
	}
	diffLong := filters.SummarizeGitDiff(strings.Join([]string{
		"diff --git a/a b/a",
		" one | 1 +",
		" two | 1 +",
		" three | 1 +",
		" four | 1 +",
		" five | 1 +",
		" six | 1 +",
		" seven | 1 +",
		" eight | 1 +",
		" nine | 1 +",
	}, "\n"))
	if strings.Count(diffLong, "\n") != 8 {
		t.Fatalf("expected truncated diff summary, got %q", diffLong)
	}

	if got := filters.SummarizeGenericFailure("", 3); got != "ok" {
		t.Fatalf("unexpected empty generic failure: %q", got)
	}
	generic := filters.SummarizeGenericFailure("info\nwarning: x\npanic: y\n", 1)
	if generic != "warning: x" {
		t.Fatalf("unexpected generic failure summary: %q", generic)
	}
	fallback := filters.SummarizeGenericFailure("line1\nline2\nline3\n", 2)
	if fallback != "line1\nline2\n... +1 more lines" {
		t.Fatalf("unexpected generic fallback: %q", fallback)
	}

	goJSON := filters.SummarizeGoTestJSON(strings.Join([]string{
		`{"Action":"pass","Package":"pkg/pass"}`,
		`{"Action":"fail","Package":"pkg/fail"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"TestOne"}`,
		`{"Action":"output","Package":"pkg/fail","Test":"TestOne","Output":"panic: boom"}`,
	}, "\n"))
	for _, want := range []string{"packages: pass=1 fail=1", "pkg/fail", "TestOne", "panic: boom"} {
		if !strings.Contains(goJSON, want) {
			t.Fatalf("expected %q in go json summary:\n%s", want, goJSON)
		}
	}
	allPass := filters.SummarizeGoTestJSON(`{"Action":"pass","Package":"pkg/pass"}`)
	if allPass != "packages: pass=1 fail=0\nall tests passed" {
		t.Fatalf("unexpected all pass summary: %q", allPass)
	}
	goJSON = filters.SummarizeGoTestJSON(strings.Join([]string{
		`{"Action":"fail","Package":"pkg/fail","Test":"One"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Two"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Three"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Four"}`,
		`{"Action":"fail","Package":"pkg/fail","Test":"Five"}`,
		`{"Action":"output","Package":"pkg/fail","Output":"not a panic"}`,
	}, "\n"))
	if !strings.Contains(goJSON, "... +1 more") {
		t.Fatalf("expected truncated go json failures: %q", goJSON)
	}
	compactFallback := filters.SummarizeGoTestJSON("not-json")
	if compactFallback != "not-json" {
		t.Fatalf("unexpected go json fallback: %q", compactFallback)
	}

	if got := filters.RenderJSONStructure([]byte("{bad")); got != "invalid json" {
		t.Fatalf("unexpected invalid json result: %q", got)
	}
	jsonShape := filters.RenderJSONStructure([]byte(`{"a":"x","b":1,"c":true,"d":null,"e":[{"z":"q"}]}`))
	for _, want := range []string{"a: string", "b: number", "c: bool", "d: null", "e: [", "z: string"} {
		if !strings.Contains(jsonShape, want) {
			t.Fatalf("expected %q in json shape:\n%s", want, jsonShape)
		}
	}
	if got := filters.RenderValueStructure(struct{}{}); got != "struct {}" {
		t.Fatalf("unexpected direct value render: %q", got)
	}
	if got := filters.RenderJSONStructure([]byte(`[]`)); got != "[]" {
		t.Fatalf("unexpected empty array shape: %q", got)
	}

	readNone := filters.ReadLevel([]byte("a\n// comment\n# hash"), "none", false, 0)
	if readNone != "a\n// comment\n# hash" {
		t.Fatalf("unexpected read none: %q", readNone)
	}
	readMinimal := filters.ReadLevel([]byte("a\n// comment\n# hash"), "minimal", false, 0)
	if readMinimal != "a" {
		t.Fatalf("unexpected read minimal: %q", readMinimal)
	}
	readAggressive := filters.ReadLevel([]byte("func x() { return 1 }\n\n# c"), "aggressive", true, 1)
	if !strings.Contains(readAggressive, "func x() { ... }") || strings.Contains(readAggressive, "... +") {
		t.Fatalf("unexpected read aggressive: %q", readAggressive)
	}
	readAggressive = filters.ReadLevel([]byte("line1\nline2\nline3"), "aggressive", false, 2)
	if !strings.Contains(readAggressive, "... +1 more lines") {
		t.Fatalf("expected aggressive max-lines truncation: %q", readAggressive)
	}
	if got := filters.CollapseBlock("plain text"); got != "plain text" {
		t.Fatalf("unexpected collapse fallback: %q", got)
	}
	if got := filters.Clip("abcdef", 3); got != "abc..." {
		t.Fatalf("unexpected clip long: %q", got)
	}
	if got := filters.Clip("abc", 5); got != "abc" {
		t.Fatalf("unexpected clip short: %q", got)
	}
	unique := filters.UniqueStrings([]string{" one ", "", "one", "two"})
	if strings.Join(unique, ",") != "one,two" {
		t.Fatalf("unexpected unique strings: %#v", unique)
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

	if got := filters.ScannerDedupe([]byte("x\nx\n")); got != "x (x2)" {
		t.Fatalf("unexpected scanner dedupe: %q", got)
	}
	if got := filters.StripANSI("\x1b[31mred\x1b[0m plain"); got != "red plain" {
		t.Fatalf("unexpected ansi strip: %q", got)
	}
	if got := filters.SummarizeGenericFailure("tiny", 10); got != "tiny" {
		t.Fatalf("unexpected generic passthrough: %q", got)
	}
}
