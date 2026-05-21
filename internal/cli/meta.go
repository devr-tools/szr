package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"szr/internal/history"
)

func (a *App) runSpread(args []string) int {
	showHistory := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown spread flag %s\n", arg)
			return 2
		}
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	summary := history.Summarize(records, 8)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
		return 0
	}

	if summary.Commands == 0 {
		fmt.Println("no tracked commands yet")
		return 0
	}

	ui := spreadUI{color: shouldColorizeStdout()}
	avgSavings := fmt.Sprintf("%.1f%%", summary.AveragePct)
	failureRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.FailureRate, summary.Failures, summary.Commands)
	fallbackRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.FallbackRate, summary.Fallbacks, summary.Commands)
	teeRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.TeeRate, summary.TeeCount, summary.Commands)
	ui.header("Spread Summary")
	ui.metric("commands", fmt.Sprintf("%d", summary.Commands), "")
	ui.metric("avg savings", avgSavings, withBar(summary.AveragePct, avgSavings, ui.color, true))
	ui.metric("tokens saved", fmt.Sprintf("%d", summary.SavedTokens), "")
	ui.metric("duration p50/p95", fmt.Sprintf("%dms / %dms", summary.DurationP50MS, summary.DurationP95MS), "")
	ui.metric("bytes read/parsed/emitted", fmt.Sprintf("%d / %d / %d", summary.RawBytesRead, summary.BytesParsed, summary.BytesEmitted), "")
	ui.metric("failure rate", failureRate, withBar(summary.FailureRate, failureRate, ui.color, false))
	ui.metric("fallback rate", fallbackRate, withBar(summary.FallbackRate, fallbackRate, ui.color, false))
	ui.metric("tee rate", teeRate, withBar(summary.TeeRate, teeRate, ui.color, false))
	if len(summary.TopCommands) > 0 {
		ui.section("top commands:")
		rows := make([][]string, 0, len(summary.TopCommands))
		for _, cmd := range summary.TopCommands {
			rows = append(rows, []string{
				cmd.Command,
				fmt.Sprintf("%d", cmd.Count),
				fmt.Sprintf("%.1f%% %s", cmd.AveragePct, progressBar(cmd.AveragePct, 12, false, true)),
				fmt.Sprintf("%d", cmd.SavedTokens),
				fmt.Sprintf("%d", cmd.RawTokens),
				fmt.Sprintf("%d", cmd.FilteredTokens),
			})
		}
		ui.table(
			[]string{"command", "count", "avg savings", "saved", "raw", "out"},
			rows,
			tableSpec{
				alignRight: map[int]bool{1: true, 3: true, 4: true, 5: true},
				maxWidth:   map[int]int{0: 30, 2: 24},
			},
		)
	}
	if len(summary.ProfileStats) > 0 {
		ui.section("profiles:")
		rows := make([][]string, 0, len(summary.ProfileStats))
		for _, stat := range summary.ProfileStats {
			rows = append(rows, []string{
				stat.Name,
				stat.Confidence,
				fmt.Sprintf("%d", stat.Commands),
				fmt.Sprintf("%d", stat.SavedTokens),
				fmt.Sprintf("%.1f%% %s", stat.AveragePct, progressBar(stat.AveragePct, 10, false, true)),
				fmt.Sprintf("%d/%dms", stat.DurationP50MS, stat.DurationP95MS),
				fmt.Sprintf("%.1f%%", stat.FailureRate),
				fmt.Sprintf("%.1f%%", stat.FallbackRate),
				fmt.Sprintf("%.1f%%", stat.TeeRate),
			})
		}
		ui.table(
			[]string{"profile", "conf", "count", "saved", "avg", "p50/p95", "fail", "fallback", "tee"},
			rows,
			tableSpec{
				alignRight: map[int]bool{2: true, 3: true, 5: true, 6: true, 7: true, 8: true},
				maxWidth:   map[int]int{0: 18, 4: 22},
			},
		)
	}
	if len(summary.FingerprintHotspots) > 0 {
		ui.section("poor savings fingerprints:")
		rows := make([][]string, 0, len(summary.FingerprintHotspots))
		for _, stat := range summary.FingerprintHotspots {
			rows = append(rows, []string{
				stat.Command,
				stat.Profile,
				fmt.Sprintf("%d", stat.Commands),
				fmt.Sprintf("%.1f%% %s", stat.AveragePct, progressBar(stat.AveragePct, 10, false, true)),
				fmt.Sprintf("%d/%dms", stat.DurationP50MS, stat.DurationP95MS),
				stat.Fingerprint,
			})
		}
		ui.table(
			[]string{"command", "profile", "count", "avg", "p50/p95", "fp"},
			rows,
			tableSpec{
				alignRight: map[int]bool{2: true, 4: true},
				maxWidth:   map[int]int{0: 30, 1: 18, 3: 22, 5: 16},
			},
		)
	}
	if len(summary.BudgetSuggestions) > 0 {
		ui.section("budget suggestions:")
		for _, suggestion := range summary.BudgetSuggestions {
			target := fmt.Sprintf("%d lines %d bytes %d tokens", suggestion.Suggested.MaxLines, suggestion.Suggested.MaxBytes, suggestion.Suggested.MaxTokens)
			fmt.Printf(
				"  - %s  profile=%s samples=%d %s/%s target=%s confidence=%s\n",
				suggestion.Command,
				suggestion.Profile,
				suggestion.Samples,
				suggestion.Direction,
				suggestion.Reason,
				target,
				suggestion.Confidence,
			)
		}
	}
	if showHistory {
		ui.section("recent:")
		for _, rec := range summary.Recent {
			fmt.Printf(
				"  - %s  %s  confidence=%s  %dms  exit=%d  fallback=%t  %.1f%%  %s\n",
				rec.Timestamp.Format(time.RFC3339),
				rec.Profile,
				rec.ProfileConfidence,
				rec.DurationMS,
				rec.ExitCode,
				rec.FallbackUsed,
				rec.SavingsPct,
				rec.Command,
			)
		}
	}
	return 0
}

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
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i := range headers {
			if i < len(row) {
				value := clipTableValue(row[i], spec.maxWidth[i])
				if len(value) > widths[i] {
					widths[i] = len(value)
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
			cells = append(cells, padCell(value, widths[i], spec.alignRight[i]))
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

func colorizeRate(value float64, enabled bool, higherIsBetter bool) string {
	text := fmt.Sprintf("%.1f%%", value)
	return colorizeTextByRate(value, text, enabled, higherIsBetter)
}

func withBar(value float64, text string, enabled bool, higherIsBetter bool) string {
	colored := colorizeTextByRate(value, text, enabled, higherIsBetter)
	return colored + " " + progressBar(value, 12, enabled, higherIsBetter)
}

func colorizeTextByRate(value float64, text string, enabled bool, higherIsBetter bool) string {
	if !enabled {
		return text
	}
	color := ansiYellow
	switch {
	case higherIsBetter && value >= 75:
		color = ansiSkyBlue
	case higherIsBetter && value >= 50:
		color = ansiSkyBlue
	case higherIsBetter:
		color = ansiYellow
	case value >= 50:
		color = ansiRed
	case value >= 15:
		color = ansiYellow
	default:
		color = ansiSkyBlue
	}
	return color + text + ansiReset
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
	bar := "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
	if !enabled {
		return bar
	}
	return colorizeTextByRate(value, bar, enabled, higherIsBetter)
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func padLeft(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return strings.Repeat(" ", width-len(value)) + value
}

func padCell(value string, width int, rightAlign bool) string {
	if rightAlign {
		return " " + padLeft(value, width) + " "
	}
	return " " + padRight(value, width) + " "
}

func clipTableValue(value string, maxWidth int) string {
	if maxWidth <= 0 || len(value) <= maxWidth {
		return value
	}
	if maxWidth <= 3 {
		return value[:maxWidth]
	}
	return value[:maxWidth-3] + "..."
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

func (a *App) runProfiles() int {
	for _, profile := range a.engine.Profiles() {
		fmt.Printf("%s\n  %s\n", profile.Name, profile.Description)
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
