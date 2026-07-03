package filters_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
)

// binaryishBlob builds control-byte-heavy content with printable needles
// embedded at arbitrary offsets, mirroring firmware images or corrupted
// captures piped through a read command.
func binaryishBlob(needles ...string) []byte {
	var out bytes.Buffer
	for i := 0; i < 4096; i++ {
		out.WriteByte(byte(i % 256))
	}
	for i, needle := range needles {
		out.WriteString(needle)
		for j := 0; j < 512; j++ {
			out.WriteByte(byte((i + j*7) % 32))
		}
	}
	return out.Bytes()
}

func TestIsBinaryish(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		data []byte
		want bool
	}{
		{name: "control-byte blob", data: binaryishBlob("MAGIC_OFFSET=0x7f3c"), want: true},
		{name: "plain source text", data: []byte(strings.Repeat("func process(batch []string) error { return nil }\n", 40)), want: false},
		{name: "multibyte text log", data: []byte(strings.Repeat("2026-07-02 ERROR 支付网关超时 🚨 retrying\n", 40)), want: false},
		{name: "empty", data: nil, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := filters.IsBinaryish(tc.data); got != tc.want {
				t.Fatalf("IsBinaryish(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestSummarizeBinaryishKeepsEmbeddedStrings pins the binary-content needle:
// printable runs inside control-byte output must stay discoverable in the
// render instead of drowning in replayed byte noise.
func TestSummarizeBinaryishKeepsEmbeddedStrings(t *testing.T) {
	t.Parallel()

	data := binaryishBlob("MAGIC_OFFSET=0x7f3c", "PKG_BUILD=acme-fw-2.4.1", "MAGIC_OFFSET=0x7f3c")
	got := filters.SummarizeBinaryish(data, 12)

	if !strings.HasPrefix(got, "binary output: ") {
		t.Fatalf("expected byte-count headline, got:\n%s", got)
	}
	for _, needle := range []string{"MAGIC_OFFSET=0x7f3c", "PKG_BUILD=acme-fw-2.4.1"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected embedded string %q in binary summary:\n%s", needle, got)
		}
	}
	if strings.Count(got, "MAGIC_OFFSET=0x7f3c") != 1 {
		t.Fatalf("expected repeated strings to be deduplicated:\n%s", got)
	}
	if len(got) >= len(data)/10 {
		t.Fatalf("expected the summary to elide the binary payload, got %d bytes for %d raw", len(got), len(data))
	}

	kind, _, requireRaw := filters.BinaryishRecoveryInfo(data, 12)
	if kind != "full-output" || !requireRaw {
		t.Fatalf("expected full-output recovery for binary content, got kind=%q requireRaw=%v", kind, requireRaw)
	}
}
