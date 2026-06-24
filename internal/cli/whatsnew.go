package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// whatsNewRelease holds the highlight bullets surfaced in the "What's New"
// menu banner for a single release.
type whatsNewRelease struct {
	version string
	bullets []string
}

// whatsNewReleases lists release highlights, newest first. Add a new entry at
// the top whenever a release ships so the menu banner reflects it.
var whatsNewReleases = []whatsNewRelease{
	{
		version: "0.7.0",
		bullets: []string{
			"Token-optimized rendering across command output",
			"Git success-path summaries for quieter status and diff",
			"Broader discovery coverage and tuning recommendations",
		},
	},
}

// latestWhatsNew returns the most recent release highlights, or nil when none
// are defined.
func latestWhatsNew() *whatsNewRelease {
	if len(whatsNewReleases) == 0 {
		return nil
	}
	return &whatsNewReleases[0]
}

// whatsNewLines renders the boxed "What's New" banner as plain (uncolored)
// lines. Returns nil when there is nothing to show.
func whatsNewLines(rel *whatsNewRelease) []string {
	if rel == nil || len(rel.bullets) == 0 {
		return nil
	}

	title := "What's New in " + formatMenuVersion(rel.version)
	bullets := make([]string, 0, len(rel.bullets))
	for _, bullet := range rel.bullets {
		bullets = append(bullets, "• "+bullet)
	}

	inner := utf8.RuneCountInString(title)
	for _, bullet := range bullets {
		if width := utf8.RuneCountInString(bullet); width > inner {
			inner = width
		}
	}

	lines := make([]string, 0, len(bullets)+4)
	lines = append(lines, "┌"+strings.Repeat("─", inner+2)+"┐")
	lines = append(lines, "│ "+padRight(title, inner)+" │")
	lines = append(lines, "├"+strings.Repeat("─", inner+2)+"┤")
	for _, bullet := range bullets {
		lines = append(lines, "│ "+padRight(bullet, inner)+" │")
	}
	lines = append(lines, "└"+strings.Repeat("─", inner+2)+"┘")
	return lines
}

// printWhatsNew prints the boxed "What's New" banner centered under the menu
// header. It is a no-op when no release highlights are defined.
func (a *App) printWhatsNew(ui spreadUI) {
	lines := whatsNewLines(latestWhatsNew())
	if len(lines) == 0 {
		return
	}
	width := a.menuHeaderWidth()
	for _, line := range lines {
		padding := 0
		if lineWidth := utf8.RuneCountInString(line); lineWidth < width {
			padding = (width - lineWidth) / 2
		}
		if ui.color {
			line = colorizeTableFrame(line)
		}
		fmt.Printf("%s%s\n", strings.Repeat(" ", padding), line)
	}
	fmt.Println()
}
