package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func (a *App) runSpread(args []string) int {
	opts, code := parseSpreadArgs(args)
	if code != 0 {
		return code
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	summaryRecords := filterSpreadRecords(records)
	summary := history.Summarize(summaryRecords, 8)
	cost := a.spreadCostReport(summary, opts)
	if opts.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(spreadJSONPayload{Summary: summary, Cost: cost})
		return 0
	}

	if summary.Commands == 0 {
		fmt.Println("no tracked commands yet")
		return 0
	}

	ui := spreadUI{color: shouldColorizeStdout()}
	savedDisplay := spreadSavedTokensDisplay(summary)
	failureRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.FailureRate, summary.Failures, summary.Commands)
	fallbackRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.FallbackRate, summary.Fallbacks, summary.Commands)
	emptyResultRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.EmptyResultRate, summary.EmptyResults, summary.Commands)
	teeRate := fmt.Sprintf("%.1f%% (%d/%d)", summary.TeeRate, summary.TeeCount, summary.Commands)
	ui.header("Spread Summary")
	ui.alignedMetrics([][2]string{
		{"szr version", formatMenuVersion(a.version)},
		{"Total commands", fmt.Sprintf("%d", summary.Commands)},
		{"Input tokens", formatCompactCount(summary.RawTokens)},
		{"Output tokens", formatCompactCount(summary.FilteredTokens)},
		{"Tokens saved", withBar(summary.FilteredSavingsPct, savedDisplay, ui.color, true)},
		{"Total exec time", formatDurationSummary(summary.TotalDurationMS, summary.Commands)},
	})
	ui.metric("p95 duration", fmt.Sprintf("%dms", summary.DurationP95MS), "")
	ui.metric("bytes (read/parsed/emitted)", fmt.Sprintf("%d / %d / %d", summary.RawBytesRead, summary.BytesParsed, summary.BytesEmitted), "")
	ui.metric("failed commands", failureRate, withBar(summary.FailureRate, failureRate, ui.color, false))
	ui.metric("fallback usage", fallbackRate, withBar(summary.FallbackRate, fallbackRate, ui.color, false))
	ui.metric("empty results", emptyResultRate, withBar(summary.EmptyResultRate, emptyResultRate, ui.color, false))
	ui.metric("tee usage", teeRate, withBar(summary.TeeRate, teeRate, ui.color, false))
	renderSpreadPassthrough(ui, summary)
	renderSpreadTopCommands(ui, summary.TopCommands)
	renderSpreadProfiles(ui, summary.ProfileStats)
	renderSpreadHotspots(ui, summary.CommandHotspots)
	renderSpreadFingerprints(ui, summary.FingerprintHotspots)
	renderSpreadBudgetSuggestions(ui, summary.BudgetSuggestions)
	renderSpreadCost(ui, cost)
	renderSpreadHistory(ui, summary.Recent, opts.showHistory)
	return 0
}

type spreadOptions struct {
	showHistory bool
	asJSON      bool
	cost        bool
	rate        float64
}

func parseSpreadArgs(args []string) (spreadOptions, int) {
	var opts spreadOptions
	i := 0
	for i < len(args) {
		next, ok := opts.consumeFlag(args, i)
		if !ok {
			return opts, 2
		}
		i = next
	}
	return opts, 0
}

func (o *spreadOptions) consumeFlag(args []string, i int) (int, bool) {
	switch arg := args[i]; {
	case arg == "--history":
		o.showHistory = true
	case arg == "--json":
		o.asJSON = true
	case arg == "--cost":
		o.cost = true
	case arg == "--rate":
		if i+1 >= len(args) {
			fmt.Fprintln(os.Stderr, "szr: --rate requires a value in USD per million input tokens")
			return 0, false
		}
		return i + 2, o.applyRate(args[i+1])
	case strings.HasPrefix(arg, "--rate="):
		return i + 1, o.applyRate(strings.TrimPrefix(arg, "--rate="))
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown spread flag %s\n", arg)
		return 0, false
	}
	return i + 1, true
}

func filterSpreadRecords(records []history.Record) []history.Record {
	filtered := make([]history.Record, 0, len(records))
	for _, rec := range records {
		if isSpreadExcludedCommand(rec.Command) {
			continue
		}
		filtered = append(filtered, rec)
	}
	return filtered
}

func isSpreadExcludedCommand(command string) bool {
	fields := strings.Fields(strings.ToLower(command))
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "szr" && len(fields) > 1 {
		fields = fields[1:]
	}
	return fields[0] == "uninstall"
}

func spreadSavedTokensDisplay(summary history.Summary) string {
	if summary.PassthroughCommands == 0 {
		return fmt.Sprintf("%s (%.1f%%)", formatCompactCount(summary.SavedTokens), summary.FilteredSavingsPct)
	}
	overallPct := percentSaved(summary.SavedTokens, summary.RawTokens)
	return fmt.Sprintf(
		"%s (%.1f%% of filtered; %.1f%% overall)",
		formatCompactCount(summary.SavedTokens),
		summary.FilteredSavingsPct,
		overallPct,
	)
}

func renderSpreadPassthrough(ui spreadUI, summary history.Summary) {
	if summary.PassthroughCommands == 0 {
		return
	}
	ui.metric(
		"proxied (unfiltered)",
		fmt.Sprintf(
			"%d commands, %s tokens - excluded from savings analysis",
			summary.PassthroughCommands,
			formatCompactCount(summary.PassthroughTokens),
		),
		"",
	)
}

func renderSpreadTopCommands(ui spreadUI, commands []history.CommandStat) {
	if len(commands) == 0 {
		return
	}
	ui.section("top commands:")
	rows := make([][]string, 0, len(commands))
	for _, cmd := range commands {
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

func renderSpreadProfiles(ui spreadUI, stats []history.ProfileStat) {
	if len(stats) == 0 {
		return
	}
	ui.section("profiles:")
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.Name,
			stat.Confidence,
			fmt.Sprintf("%d", stat.Commands),
			fmt.Sprintf("%d", stat.SavedTokens),
			fmt.Sprintf("%.1f%% %s", stat.AveragePct, progressBar(stat.AveragePct, 10, false, true)),
			fmt.Sprintf("%dms", stat.DurationP95MS),
			fmt.Sprintf("%.1f%%", stat.FailureRate),
			fmt.Sprintf("%.1f%%", stat.FallbackRate),
			fmt.Sprintf("%.1f%%", stat.EmptyResultRate),
			fmt.Sprintf("%.1f%%", stat.TeeRate),
		})
	}
	ui.table(
		[]string{"profile", "conf", "count", "saved", "avg", "p95", "fail", "fallback", "empty", "tee"},
		rows,
		tableSpec{
			alignRight: map[int]bool{2: true, 3: true, 5: true, 6: true, 7: true, 8: true, 9: true},
			maxWidth:   map[int]int{0: 18, 4: 22},
		},
	)
}

func renderSpreadHotspots(ui spreadUI, stats []history.CommandHotspot) {
	if len(stats) == 0 {
		return
	}
	ui.section("improvement hotspots:")
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.Command,
			stat.Profile,
			fmt.Sprintf("%d", stat.Commands),
			fmt.Sprintf("%.1f%% %s", stat.AveragePct, progressBar(stat.AveragePct, 10, false, true)),
			fmt.Sprintf("%.1f%%", stat.FallbackRate),
			fmt.Sprintf("%dms", stat.DurationP95MS),
		})
	}
	ui.table(
		[]string{"command", "profile", "count", "avg", "fallback", "p95"},
		rows,
		tableSpec{
			alignRight: map[int]bool{2: true, 4: true, 5: true},
			maxWidth:   map[int]int{0: 24, 1: 16, 3: 22},
		},
	)
	top := stats[0]
	fmt.Printf("  top action: %s for %s (%s)\n", hotspotAction(top), top.Command, top.Profile)
}

func renderSpreadFingerprints(ui spreadUI, stats []history.FingerprintStat) {
	if len(stats) == 0 {
		return
	}
	ui.section("poor savings fingerprints:")
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, []string{
			stat.Command,
			stat.Profile,
			fmt.Sprintf("%d", stat.Commands),
			fmt.Sprintf("%.1f%% %s", stat.AveragePct, progressBar(stat.AveragePct, 10, false, true)),
			fmt.Sprintf("%dms", stat.DurationP95MS),
			stat.Fingerprint,
		})
	}
	ui.table(
		[]string{"command", "profile", "count", "avg", "p95", "fp"},
		rows,
		tableSpec{
			alignRight: map[int]bool{2: true, 4: true},
			maxWidth:   map[int]int{0: 30, 1: 18, 3: 22, 5: 16},
		},
	)
}

func renderSpreadBudgetSuggestions(ui spreadUI, suggestions []history.BudgetSuggestion) {
	if len(suggestions) == 0 {
		return
	}
	ui.section("budget suggestions:")
	for _, suggestion := range suggestions {
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

func hotspotAction(stat history.CommandHotspot) string {
	switch {
	case stat.FallbackRate >= 20:
		return "loosen budget or improve fallback path"
	case stat.AveragePct <= 0:
		return "tiny output overhead; bypass or shorten summary"
	case stat.AveragePct <= 20:
		return "tighten reducer or prefer a terser mode"
	case stat.AveragePct <= 35:
		return "review budget before adding more structure"
	default:
		return "monitor"
	}
}

func renderSpreadHistory(ui spreadUI, recent []history.Record, showHistory bool) {
	if !showHistory {
		return
	}
	ui.section("recent:")
	for _, rec := range recent {
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

func percentSaved(saved, raw int) float64 {
	if raw <= 0 {
		return 0
	}
	return float64(saved) * 100 / float64(raw)
}

func formatCompactCount(value int) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	switch {
	case value >= 1_000_000:
		return fmt.Sprintf("%s%.1fM", sign, float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%s%.1fK", sign, float64(value)/1_000)
	default:
		return fmt.Sprintf("%s%d", sign, value)
	}
}

func formatDurationSummary(totalMS int64, commands int) string {
	total := time.Duration(totalMS) * time.Millisecond
	avg := time.Duration(0)
	if commands > 0 {
		avg = total / time.Duration(commands)
	}
	return fmt.Sprintf("%s (avg %s)", formatRoundedDuration(total), formatAverageDuration(avg))
}

func formatRoundedDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value/time.Millisecond)
	}
	value = value.Round(time.Second)
	hours := value / time.Hour
	value %= time.Hour
	minutes := value / time.Minute
	value %= time.Minute
	seconds := value / time.Second
	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

func formatAverageDuration(value time.Duration) string {
	if value < time.Second {
		return fmt.Sprintf("%dms", value/time.Millisecond)
	}
	return fmt.Sprintf("%.1fs", value.Seconds())
}
