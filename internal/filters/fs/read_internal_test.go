package fs

import (
	"strings"
	"testing"
)

// TestSummarizeReadFileBinaryish pins the binary read path: reading a
// control-byte file must render the byte count plus its embedded printable
// strings instead of replaying byte noise as pseudo-code lines.
func TestSummarizeReadFileBinaryish(t *testing.T) {
	data := make([]byte, 0, 6000)
	for i := 0; i < 5000; i++ {
		data = append(data, byte(i%32))
	}
	data = append(data, []byte("MAGIC_OFFSET=0x7f3c")...)
	for i := 0; i < 500; i++ {
		data = append(data, byte(i%16))
	}

	got := SummarizeReadFile("firmware.bin", data, 12)
	if !strings.HasPrefix(got, "binary output: ") {
		t.Fatalf("expected binary headline for control-byte file, got:\n%s", got)
	}
	if !strings.Contains(got, "MAGIC_OFFSET=0x7f3c") {
		t.Fatalf("expected embedded string needle in binary read preview:\n%s", got)
	}

	kind, _, requireRaw := ReadFileRecoveryInfo("firmware.bin", data, 12)
	if kind != "full-output" || !requireRaw {
		t.Fatalf("expected full-output recovery for binary read, got kind=%q requireRaw=%v", kind, requireRaw)
	}
}

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
