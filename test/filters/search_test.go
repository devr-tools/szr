package filters_test

import (
	"errors"
	"strings"
	"testing"

	"szr/internal/filters"
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
	for _, want := range []string{"one.go (4 matches)", "  ... +1 more", "... +1 more files"} {
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

func TestSummarizeRipgrep(t *testing.T) {
	grouped := filters.SummarizeRipgrep(strings.Join([]string{
		"one.go:1:first",
		"one.go:2:second",
		"two.go:9:two",
	}, "\n"), 4, 6)
	for _, want := range []string{"one.go (2 matches)", "two.go (1 matches)"} {
		if !strings.Contains(grouped, want) {
			t.Fatalf("expected %q in ripgrep summary:\n%s", want, grouped)
		}
	}

	fallback := filters.SummarizeRipgrep("rg: ./vendor: Permission denied (os error 13)\n", 4, 4)
	if !strings.Contains(fallback, "Permission denied") {
		t.Fatalf("expected ripgrep error fallback, got %q", fallback)
	}
}
