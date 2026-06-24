package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLatestWhatsNew(t *testing.T) {
	rel := latestWhatsNew()
	if rel == nil {
		t.Fatal("expected at least one release in whatsNewReleases")
	}
	if rel.version != whatsNewReleases[0].version {
		t.Fatalf("latestWhatsNew returned %q, want %q", rel.version, whatsNewReleases[0].version)
	}
}

func TestWhatsNewLinesNilWhenEmpty(t *testing.T) {
	if lines := whatsNewLines(nil); lines != nil {
		t.Fatalf("expected nil for nil release, got %v", lines)
	}
	if lines := whatsNewLines(&whatsNewRelease{version: "1.0.0"}); lines != nil {
		t.Fatalf("expected nil for release without bullets, got %v", lines)
	}
}

func TestWhatsNewLinesStructureAndAlignment(t *testing.T) {
	rel := &whatsNewRelease{
		version: "0.7.0",
		bullets: []string{"short", "a noticeably longer highlight line"},
	}
	lines := whatsNewLines(rel)

	// border + title + separator + 2 bullets + bottom border
	if len(lines) != 5+1 {
		t.Fatalf("expected 6 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[1], "What's New in v0.7.0") {
		t.Fatalf("title line missing version: %q", lines[1])
	}

	// All lines must be the same display width so the box stays aligned.
	width := utf8.RuneCountInString(lines[0])
	for i, line := range lines {
		if got := utf8.RuneCountInString(line); got != width {
			t.Fatalf("line %d width %d, want %d: %q", i, got, width, line)
		}
	}

	if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
		t.Fatalf("unexpected top border: %q", lines[0])
	}
	if !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Fatalf("unexpected bottom border: %q", lines[len(lines)-1])
	}
}
