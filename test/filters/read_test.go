package filters_test

import (
	"strings"
	"testing"

	"szr/internal/filters"
)

func TestReadLevels(t *testing.T) {
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
}

func TestSummarizeReadFile(t *testing.T) {
	doc := filters.SummarizeReadFile("README.md", []byte("# Title\n\n- one\nBody\n"), 3)
	for _, want := range []string{"# Title", "- one"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected %q in doc preview %q", want, doc)
		}
	}

	code := filters.SummarizeReadFile("main.go", []byte("package main\n\n// comment\nfunc main() { println(\"x\") }\n"), 3)
	for _, want := range []string{"1  package main", "4  func main() { ... }"} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected %q in code preview %q", want, code)
		}
	}

	jsonPreview := filters.SummarizeReadFile("cfg.json", []byte(`{"name":"x","items":[{"id":1}]}`), 8)
	if !strings.Contains(jsonPreview, "name: string") || !strings.Contains(jsonPreview, "id: number") {
		t.Fatalf("expected json structure preview, got %q", jsonPreview)
	}
}

func TestSummarizeDirectoryAndTree(t *testing.T) {
	listing := filters.SummarizeDirectoryListing("README.md\nMakefile\nsrc/\ndocs/\ninternal/\ntest/\n", 4)
	if !strings.Contains(listing, "dirs:") || !strings.Contains(listing, "files:") {
		t.Fatalf("expected grouped directory listing, got %q", listing)
	}

	tree := filters.SummarizeTreeOutput(strings.Join([]string{
		"project",
		"|-- cmd",
		"|   |-- atlas",
		"|   `-- atlas-dev",
		"|-- docs",
		"|   |-- architecture.md",
		"|   `-- benchmark.md",
		"`-- README.md",
		"",
		"3 directories, 5 files",
	}, "\n"), 5)
	for _, want := range []string{"project", "cmd (2)", "docs (2)", "3 directories, 5 files"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("expected %q in tree summary %q", want, tree)
		}
	}
}
