package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
	fsfilter "github.com/devr-tools/szr/internal/filters/fs"
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
	readMinimalLimited := filters.ReadLevel([]byte("a\nb\nc"), "minimal", false, 2)
	if readMinimalLimited != "a\nb\n... +1 more lines" {
		t.Fatalf("unexpected declarative read minimal limit: %q", readMinimalLimited)
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
	doc := fsfilter.SummarizeReadFile("README.md", []byte("# Title\n\n- one\nBody\n"), 3)
	for _, want := range []string{"# Title", "- one"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected %q in doc preview %q", want, doc)
		}
	}

	code := fsfilter.SummarizeReadFile("main.go", []byte("package main\n\n// comment\nfunc main() { println(\"x\") }\n"), 3)
	for _, want := range []string{"1  package main", "4  func main() { ... }"} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected %q in code preview %q", want, code)
		}
	}

	nonSignatureCode := fsfilter.SummarizeReadFile("script.py", []byte("value = 1\nprint(value)\n# tail\n"), 3)
	for _, want := range []string{"1  value = 1", "2  print(value)"} {
		if !strings.Contains(nonSignatureCode, want) {
			t.Fatalf("expected %q in non-signature code preview %q", want, nonSignatureCode)
		}
	}

	collapsedPreview := fsfilter.SummarizeReadFile("settings.conf", []byte("route = { keep = 1, drop = 2 }\nplain = 1\n"), 3)
	for _, want := range []string{"1  route = { ... }", "2  plain = 1"} {
		if !strings.Contains(collapsedPreview, want) {
			t.Fatalf("expected %q in collapsed non-signature preview %q", want, collapsedPreview)
		}
	}

	jsonPreview := fsfilter.SummarizeReadFile("cfg.json", []byte(`{"name":"x","items":[{"id":1}]}`), 8)
	if !strings.Contains(jsonPreview, "name: string") || !strings.Contains(jsonPreview, "id: number") {
		t.Fatalf("expected json structure preview, got %q", jsonPreview)
	}

	if kind, summary, requireRawCapture := fsfilter.ReadFileRecoveryInfo("README.md", []byte("# Title\n\n- one\nBody\nAnother body line\n"), 2); kind != filters.RecoveryKindFullOutput || summary != "omitted 2 additional lines" || !requireRawCapture {
		t.Fatalf("unexpected read file recovery info: kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}

	if kind, summary, requireRawCapture := fsfilter.ReadFileRecoveryInfo("cfg.json", []byte(`{"name":"x","items":[{"id":1},{"id":2},{"id":3}],"meta":{"env":"dev","owner":"team","service":"api"},"flags":{"a":true,"b":false},"nested":{"child":{"leaf":"x"}}}`), 2); kind != filters.RecoveryKindFullOutput || summary == "" || !requireRawCapture {
		t.Fatalf("expected json read preview recovery info, got kind=%q summary=%q requireRawCapture=%v", kind, summary, requireRawCapture)
	}
}

func TestSummarizeReadFileCodeSignatureMode(t *testing.T) {
	code := fsfilter.SummarizeReadFile("sample.go", []byte(strings.Join([]string{
		"package sample",
		"",
		"import (",
		"\t\"context\"",
		"\t\"fmt\"",
		")",
		"// TODO: keep exported surface small",
		"const (",
		"\tExportedThing = \"x\"",
		"\tinternalThing = \"y\"",
		")",
		"func Run(",
		"\tctx context.Context,",
		"\tvalue string,",
		") error {",
		"\tfmt.Println(value)",
		"\treturn nil",
		"}",
		"type service struct {",
		"\tname string",
		"}",
	}, "\n")), 12)

	for _, want := range []string{
		"1  package sample",
		"3  import (",
		"4  \"context\"",
		"5  \"fmt\"",
		"7  TODO: keep exported surface small",
		"8  const ( ... )",
		"9  ExportedThing",
		"12  func Run( ctx context.Context, value string, ) error { ... }",
		"19  type service struct { ... }",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("expected %q in signature preview %q", want, code)
		}
	}

	for _, unwanted := range []string{"fmt.Println(value)", "internalThing", "name string"} {
		if strings.Contains(code, unwanted) {
			t.Fatalf("did not expect %q in signature preview %q", unwanted, code)
		}
	}

	fallback := fsfilter.SummarizeReadFile("boring.go", []byte(strings.Join([]string{
		"value := 1",
		"println(value)",
	}, "\n")), 2)
	if !strings.Contains(fallback, "value := 1") || !strings.Contains(fallback, "println(value)") {
		t.Fatalf("expected fallback signature preview to preserve aggressive lines, got %q", fallback)
	}

	script := fsfilter.SummarizeReadFile("script.sh", []byte(strings.Join([]string{
		"#!/usr/bin/env bash",
		"# TODO: keep startup cheap",
		"source ./common.sh",
		"export NAME=value",
		"type (",
		"\tExportedThing string",
		"\t// TODO: leave a breadcrumb",
		"\tinternalThing string",
		")",
	}, "\n")), 12)
	for _, want := range []string{
		"1  #!/usr/bin/env bash",
		"2  TODO: keep startup cheap",
		"4  export NAME=value",
		"5  type ( ... )",
		"6  ExportedThing",
		"7  TODO: leave a breadcrumb",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in script signature preview %q", want, script)
		}
	}
}

func TestSummarizeDirectoryAndTree(t *testing.T) {
	listing := fsfilter.SummarizeDirectoryListing("README.md\nMakefile\nsrc/\ndocs/\ninternal/\ntest/\n", 4)
	if !strings.Contains(listing, "dirs:") || !strings.Contains(listing, "files:") {
		t.Fatalf("expected grouped directory listing, got %q", listing)
	}

	tree := fsfilter.SummarizeTreeOutput(strings.Join([]string{
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
