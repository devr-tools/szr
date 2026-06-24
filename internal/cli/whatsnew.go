package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type whatsNewRelease struct {
	version string
	bullets []string
}

// whatsNewReleases is newest-first; add an entry at the top for each release.
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

func latestWhatsNew() *whatsNewRelease {
	if len(whatsNewReleases) == 0 {
		return nil
	}
	return &whatsNewReleases[0]
}

func whatsNewLines(rel *whatsNewRelease) []string {
	if rel == nil || len(rel.bullets) == 0 {
		return nil
	}

	title := "What's New in " + formatMenuVersion(rel.version)
	bullets := whatsNewBullets(rel.bullets)
	return whatsNewBoxLines(title, bullets)
}

func whatsNewBullets(bullets []string) []string {
	lines := make([]string, 0, len(bullets))
	for _, bullet := range bullets {
		lines = append(lines, "• "+bullet)
	}
	return lines
}

func whatsNewBoxLines(title string, bullets []string) []string {
	inner := whatsNewInnerWidth(title, bullets)
	lines := make([]string, 0, len(bullets)+4)
	border := strings.Repeat("─", inner+2)

	lines = append(lines, "┌"+border+"┐")
	lines = append(lines, whatsNewContentLine(title, inner))
	lines = append(lines, "├"+border+"┤")
	for _, bullet := range bullets {
		lines = append(lines, whatsNewContentLine(bullet, inner))
	}
	lines = append(lines, "└"+border+"┘")
	return lines
}

func whatsNewInnerWidth(title string, bullets []string) int {
	inner := utf8.RuneCountInString(title)
	for _, bullet := range bullets {
		if width := utf8.RuneCountInString(bullet); width > inner {
			inner = width
		}
	}
	return inner
}

func whatsNewContentLine(value string, width int) string {
	return "│ " + padRight(value, width) + " │"
}

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
