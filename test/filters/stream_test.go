package filters_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/filters"
)

func TestStripANSIOSCHyperlink(t *testing.T) {
	t.Parallel()

	// OSC 8 hyperlink terminated by ST (ESC \): only the link text survives.
	input := "\x1b]8;;https://example.com\x1b\\text\x1b]8;;\x1b\\"
	if got := filters.StripANSI(input); got != "text" {
		t.Fatalf("unexpected osc hyperlink strip: %q", got)
	}
}

func TestStripANSIOSCBELTerminator(t *testing.T) {
	t.Parallel()

	// OSC 0 (window title) terminated by BEL.
	if got := filters.StripANSI("\x1b]0;my title\x07after"); got != "after" {
		t.Fatalf("unexpected osc bel strip: %q", got)
	}
	// Hyperlink variant with BEL terminators.
	input := "\x1b]8;;https://example.com\x07link\x1b]8;;\x07 done"
	if got := filters.StripANSI(input); got != "link done" {
		t.Fatalf("unexpected osc bel hyperlink strip: %q", got)
	}
}

func TestStripANSICSI(t *testing.T) {
	t.Parallel()

	if got := filters.StripANSI("\x1b[31mred\x1b[0m plain"); got != "red plain" {
		t.Fatalf("unexpected csi strip: %q", got)
	}
	// CSI with multiple parameters and an intermediate byte.
	if got := filters.StripANSI("\x1b[1;38;5;196mbold\x1b[!pdone"); got != "bolddone" {
		t.Fatalf("unexpected extended csi strip: %q", got)
	}
}

func TestStripANSITwoByteEscapes(t *testing.T) {
	t.Parallel()

	// ESC 7 / ESC 8 (save/restore cursor) are two-byte escapes; the old
	// stripper scanned for a letter and swallowed following text.
	if got := filters.StripANSI("\x1b7save\x1b8restore"); got != "saverestore" {
		t.Fatalf("unexpected two-byte escape strip: %q", got)
	}
	// ESC M (reverse index) is a letter-final two-byte escape.
	if got := filters.StripANSI("\x1bMup"); got != "up" {
		t.Fatalf("unexpected esc-m strip: %q", got)
	}
	// Charset designation uses an intermediate byte: ESC ( B.
	if got := filters.StripANSI("\x1b(Bascii"); got != "ascii" {
		t.Fatalf("unexpected charset designation strip: %q", got)
	}
}

func TestStripANSIStringSequences(t *testing.T) {
	t.Parallel()

	// DCS, PM, and APC payloads terminate on ST just like OSC.
	if got := filters.StripANSI("\x1bPpayload\x1b\\dcs \x1b^pm\x1b\\pm \x1b_apc\x1b\\apc"); got != "dcs pm apc" {
		t.Fatalf("unexpected string sequence strip: %q", got)
	}
}

func TestLineScannerOSCSplitAcrossChunks(t *testing.T) {
	t.Parallel()

	var scanner filters.LineScanner
	var lines []string
	emit := func(line string) { lines = append(lines, line) }
	scanner.Consume([]byte("foo\x1b]8;;https://exam"), emit)
	scanner.Consume([]byte("ple.com/path\x1b\\bar\n"), emit)
	scanner.Finish(emit)
	if strings.Join(lines, "|") != "foobar" {
		t.Fatalf("unexpected chunked osc scan: %q", lines)
	}
}
