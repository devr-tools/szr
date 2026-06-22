package fs

import "testing"

func TestNormalizePreviewLineCollapsesInlineBlocks(t *testing.T) {
	if got := normalizePreviewLine("value = { keep: 1, drop: 2 }"); got != "value = { ... }" {
		t.Fatalf("unexpected collapsed preview line: %q", got)
	}
	if got := normalizePreviewLine("value = 1"); got != "value = 1" {
		t.Fatalf("unexpected plain preview line: %q", got)
	}
}

func TestAppendDeclarationBlockLine(t *testing.T) {
	out := []previewLine{}
	appendDeclarationBlockLine(&out, 0, "")
	appendDeclarationBlockLine(&out, 1, "// TODO: keep exported surface tight")
	appendDeclarationBlockLine(&out, 2, "ExportedThing string")
	appendDeclarationBlockLine(&out, 3, "internalThing string")

	if len(out) != 2 {
		t.Fatalf("unexpected declaration block entries: %#v", out)
	}
	if out[0].Text != "TODO: keep exported surface tight" {
		t.Fatalf("unexpected TODO entry: %#v", out[0])
	}
	if out[1].Text != "ExportedThing" {
		t.Fatalf("unexpected exported member entry: %#v", out[1])
	}
}

func TestDeclarationSignatureState(t *testing.T) {
	if skip, stop := declarationSignatureState("", 0, 0); !skip || stop {
		t.Fatalf("unexpected initial empty signature state: skip=%v stop=%v", skip, stop)
	}
	if skip, stop := declarationSignatureState("", 1, 0); skip || !stop {
		t.Fatalf("unexpected later empty signature state: skip=%v stop=%v", skip, stop)
	}
	if skip, stop := declarationSignatureState("// comment", 2, 0); !skip || stop {
		t.Fatalf("unexpected comment-only signature state: skip=%v stop=%v", skip, stop)
	}
	if skip, stop := declarationSignatureState("// TODO: keep it", 2, 0); skip || stop {
		t.Fatalf("unexpected todo signature state: skip=%v stop=%v", skip, stop)
	}
}

func TestRenderDeclarationBlockHeader(t *testing.T) {
	if got := renderDeclarationBlockHeader("type ("); got != "type ( ... )" {
		t.Fatalf("unexpected paren block header: %q", got)
	}
	if got := renderDeclarationBlockHeader("struct {"); got != "struct { ... }" {
		t.Fatalf("unexpected brace block header: %q", got)
	}
	if got := renderDeclarationBlockHeader("const Demo"); got != "const Demo" {
		t.Fatalf("unexpected plain block header: %q", got)
	}
}
