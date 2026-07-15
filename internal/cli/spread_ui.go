package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/devr-tools/szr/internal/engine"
)

type spreadUI struct {
	color bool
}

func (ui spreadUI) header(title string) {
	line := strings.Repeat("=", len(title)+4)
	if ui.color {
		line = ansiBold + ansiSkyBlue + line + ansiReset
		title = ansiBold + title + ansiReset
	}
	fmt.Println(line)
	fmt.Printf("  %s\n", title)
	fmt.Println(line)
}

func (ui spreadUI) section(title string) {
	if ui.color {
		title = ansiBold + ansiSkyBlue + title + ansiReset
	}
	fmt.Println()
	fmt.Println(title)
}

func (ui spreadUI) metric(label, value, coloredValue string) {
	if coloredValue != "" {
		value = coloredValue
	}
	displayLabel := label + ":"
	if ui.color {
		displayLabel = ansiDim + displayLabel + ansiReset
	}
	fmt.Printf("%s %s\n", displayLabel, value)
}

func (ui spreadUI) alignedMetrics(rows [][2]string) {
	if len(rows) == 0 {
		return
	}
	width := 0
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	for _, row := range rows {
		label := row[0] + ":"
		label = padRight(label, width+1)
		if ui.color {
			label = ansiDim + label + ansiReset
		}
		fmt.Printf("%s %s\n", label, row[1])
	}
}

type tableSpec struct {
	alignRight map[int]bool
	maxWidth   map[int]int
}

func (ui spreadUI) table(headers []string, rows [][]string, spec tableSpec) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = utf8.RuneCountInString(header)
	}
	for _, row := range rows {
		for i := range headers {
			if i < len(row) {
				value := clipTableValue(row[i], spec.maxWidth[i])
				if width := utf8.RuneCountInString(value); width > widths[i] {
					widths[i] = width
				}
			}
		}
	}

	top := make([]string, 0, len(headers))
	mid := make([]string, 0, len(headers))
	bot := make([]string, 0, len(headers))
	for i := range headers {
		top = append(top, strings.Repeat("─", widths[i]+2))
		mid = append(mid, strings.Repeat("─", widths[i]+2))
		bot = append(bot, strings.Repeat("─", widths[i]+2))
	}
	topLine := "  ┌" + strings.Join(top, "┬") + "┐"
	headerCells := make([]string, 0, len(headers))
	for i, header := range headers {
		headerCells = append(headerCells, padCell(header, widths[i], false))
	}
	headerLine := "  │" + strings.Join(headerCells, "│") + "│"
	midLine := "  ├" + strings.Join(mid, "┼") + "┤"
	if ui.color {
		topLine = ansiSkyBlue + topLine + ansiReset
		headerLine = colorizeTableFrame(headerLine)
		midLine = ansiSkyBlue + midLine + ansiReset
	}
	fmt.Println(topLine)
	fmt.Println(headerLine)
	fmt.Println(midLine)
	for _, row := range rows {
		cells := make([]string, 0, len(headers))
		for i := range headers {
			value := ""
			if i < len(row) {
				value = clipTableValue(row[i], spec.maxWidth[i])
			}
			cells = append(cells, colorizeEmbeddedBar(padCell(value, widths[i], spec.alignRight[i]), ui.color))
		}
		rowLine := "  │" + strings.Join(cells, "│") + "│"
		if ui.color {
			rowLine = colorizeTableFrame(rowLine)
		}
		fmt.Println(rowLine)
	}
	bottomLine := "  └" + strings.Join(bot, "┴") + "┘"
	if ui.color {
		bottomLine = ansiSkyBlue + bottomLine + ansiReset
	}
	fmt.Println(bottomLine)
}

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiGreen   = "\033[32m"
	ansiRed     = "\033[31m"
	ansiYellow  = "\033[33m"
	ansiSkyBlue = "\033[38;2;32;171;246m"
)

func shouldColorizeStdout() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("TERM"), "dumb") {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func colorizeTextByRate(value float64, text string, enabled bool, higherIsBetter bool) string {
	if !enabled {
		return text
	}
	color := ansiYellow
	switch {
	case higherIsBetter && value > 0:
		color = ansiGreen
	case higherIsBetter && value < 0:
		color = ansiRed
	case !higherIsBetter && value >= 50:
		color = ansiRed
	case !higherIsBetter && value >= 15:
		color = ansiYellow
	case !higherIsBetter:
		color = ansiGreen
	}
	return color + text + ansiReset
}

func withBar(value float64, text string, enabled bool, higherIsBetter bool) string {
	colored := colorizeTextByRate(value, text, enabled, higherIsBetter)
	return colored + " " + progressBar(value, 12, enabled, higherIsBetter)
}

func progressBar(value float64, width int, enabled bool, higherIsBetter bool) string {
	if width <= 0 {
		width = 10
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	filled := int((value*float64(width) + 50) / 100)
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	if !enabled {
		return bar
	}
	return colorizeTextByRate(value, bar, enabled, higherIsBetter)
}

func padRight(value string, width int) string {
	if utf8.RuneCountInString(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-utf8.RuneCountInString(value))
}

func padLeft(value string, width int) string {
	if utf8.RuneCountInString(value) >= width {
		return value
	}
	return strings.Repeat(" ", width-utf8.RuneCountInString(value)) + value
}

func padCell(value string, width int, rightAlign bool) string {
	if rightAlign {
		return " " + padLeft(value, width) + " "
	}
	return " " + padRight(value, width) + " "
}

func clipTableValue(value string, maxWidth int) string {
	if maxWidth <= 0 || utf8.RuneCountInString(value) <= maxWidth {
		return value
	}
	runes := []rune(value)
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}
	return string(runes[:maxWidth-3]) + "..."
}

func colorizeTableFrame(line string) string {
	var out strings.Builder
	for _, r := range line {
		switch r {
		case '│', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼':
			out.WriteString(ansiSkyBlue)
			out.WriteRune(r)
			out.WriteString(ansiReset)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func colorizeEmbeddedBar(value string, enabled bool) string {
	if !enabled {
		return value
	}
	pctIndex := strings.Index(value, "%")
	if pctIndex < 0 {
		return value
	}
	barStart := pctIndex + 1
	for barStart < len(value) && value[barStart] == ' ' {
		barStart++
	}
	if barStart >= len(value) {
		return value
	}
	numberText := strings.TrimSpace(value[:pctIndex])
	parts := strings.Fields(numberText)
	if len(parts) == 0 {
		return value
	}
	rate, err := strconv.ParseFloat(parts[len(parts)-1], 64)
	if err != nil {
		return value
	}
	barEnd := barStart
	for barEnd < len(value) {
		r, size := utf8.DecodeRuneInString(value[barEnd:])
		if !isMeterRune(r) {
			break
		}
		barEnd += size
	}
	if barEnd == barStart {
		return value
	}
	bar := value[barStart:barEnd]
	return value[:barStart] + colorizeTextByRate(rate, bar, enabled, true) + value[barEnd:]
}

func isMeterRune(r rune) bool {
	return r == '█' || r == '░'
}

func formatTokenCount(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	text := strconv.Itoa(value)
	if len(text) <= 3 {
		return sign + text + " tokens"
	}
	var parts []string
	for len(text) > 3 {
		parts = append([]string{text[len(text)-3:]}, parts...)
		text = text[:len(text)-3]
	}
	parts = append([]string{text}, parts...)
	return sign + strings.Join(parts, ",") + " tokens"
}

func (a *App) runProfiles() int {
	for _, profile := range a.engine.Profiles() {
		fmt.Printf("%s\n  %s\n", profile.Name, profile.Description)
		if profile.Source != "" && profile.Source != engine.SourceBuiltin {
			fmt.Printf("  source: %s\n", profile.Source)
		}
		if profile.Confidence != "" {
			fmt.Printf("  confidence: %s\n", profile.Confidence)
		}
		if profile.StreamPreference != "" {
			fmt.Printf("  stream: %s\n", profile.StreamPreference)
		}
		if profile.Budget.MaxLines > 0 || profile.Budget.MaxBytes > 0 || profile.Budget.MaxTokens > 0 {
			fmt.Printf("  budget: lines=%d bytes=%d tokens=%d\n", profile.Budget.MaxLines, profile.Budget.MaxBytes, profile.Budget.MaxTokens)
		}
		if profile.Budget.MinFailures > 0 || profile.Budget.MinAnchors > 0 || profile.Budget.MinHints > 0 {
			fmt.Printf("  contract: failures=%d anchors=%d hints=%d\n", profile.Budget.MinFailures, profile.Budget.MinAnchors, profile.Budget.MinHints)
		}
		if profile.LatencyBudget > 0 {
			fmt.Printf("  latency budget: %s\n", profile.LatencyBudget.Round(time.Millisecond))
		}
	}
	return 0
}
