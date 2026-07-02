package cli

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	szrroot "github.com/devr-tools/szr"
)

const (
	whatsNewMaxBullets     = 3
	whatsNewMaxBulletWidth = 60
)

type whatsNewRelease struct {
	version string
	bullets []string
}

// latestWhatsNew parses the newest release section out of the embedded
// changelog, which release-please rewrites on every release.
var latestWhatsNew = sync.OnceValue(func() *whatsNewRelease {
	return parseLatestWhatsNew(szrroot.Changelog)
})

var (
	whatsNewVersionPattern  = regexp.MustCompile(`^##\s+\[?v?(\d+\.\d+\.\d+)`)
	whatsNewMarkdownLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	whatsNewTrailingCommits = regexp.MustCompile(`(?:\s*\((?:[0-9a-f]{7,40}|#\d+)(?:,\s*(?:[0-9a-f]{7,40}|#\d+))*\)|,?\s*closes\s+#\d+(?:,\s*#\d+)*)\s*$`)
)

func parseLatestWhatsNew(changelog string) *whatsNewRelease {
	version := ""
	section := ""
	var features, fixes, others []string
	for _, line := range strings.Split(changelog, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := whatsNewVersionPattern.FindStringSubmatch(trimmed); match != nil {
			if version != "" {
				break
			}
			version = match[1]
			continue
		}
		if version == "" {
			continue
		}
		if after, ok := strings.CutPrefix(trimmed, "### "); ok {
			section = strings.TrimSpace(after)
			continue
		}
		bulletText, ok := strings.CutPrefix(trimmed, "* ")
		if !ok {
			continue
		}
		bullet := cleanWhatsNewBullet(bulletText)
		if bullet == "" {
			continue
		}
		switch section {
		case "Features":
			features = append(features, bullet)
		case "Bug Fixes":
			fixes = append(fixes, bullet)
		default:
			others = append(others, bullet)
		}
	}
	if version == "" {
		return nil
	}
	bullets := dedupeWhatsNewBullets(append(append(features, fixes...), others...), whatsNewMaxBullets)
	if len(bullets) == 0 {
		return nil
	}
	return &whatsNewRelease{version: version, bullets: bullets}
}

func cleanWhatsNewBullet(text string) string {
	text = whatsNewMarkdownLink.ReplaceAllString(text, "$1")
	for {
		stripped := whatsNewTrailingCommits.ReplaceAllString(text, "")
		if stripped == text {
			break
		}
		text = stripped
	}
	text = strings.ReplaceAll(text, "**", "")
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	first, size := utf8.DecodeRuneInString(text)
	text = string(unicode.ToUpper(first)) + text[size:]
	if utf8.RuneCountInString(text) > whatsNewMaxBulletWidth {
		text = string([]rune(text)[:whatsNewMaxBulletWidth-1]) + "…"
	}
	return text
}

func dedupeWhatsNewBullets(bullets []string, limit int) []string {
	seen := make(map[string]bool, len(bullets))
	kept := make([]string, 0, limit)
	for _, bullet := range bullets {
		key := strings.ToLower(bullet)
		if seen[key] {
			continue
		}
		seen[key] = true
		kept = append(kept, bullet)
		if len(kept) == limit {
			break
		}
	}
	return kept
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
