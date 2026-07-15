package cli

import "fmt"

func renderUsageReport(report usageReport) {
	ui := spreadUI{color: shouldColorizeStdout()}
	ui.header("Usage Summary")
	ui.alignedMetrics([][2]string{
		{"Sessions", fmt.Sprintf("%d", report.Totals.Sessions)},
		{"Model turns", fmt.Sprintf("%d", report.Totals.Turns)},
		{"Model fresh input", formatCompactCount(report.Totals.FreshInputTokens)},
		{"Model cache reads", formatCompactCount(report.Totals.CacheReadTokens)},
		{"Model output", formatCompactCount(report.Totals.OutputTokens)},
		{"szr commands", fmt.Sprintf("%d", report.Totals.SZRCommands)},
		{"szr emitted (est.)", formatCompactCount(report.Totals.SZREmittedTokens)},
		{"szr avoided (est.)", formatCompactCount(report.Totals.SZRAvoidedTokens)},
	})
	renderUsageTable(ui, report)
	renderUsageNotes(report.Notes)
}

func renderUsageTable(ui spreadUI, report usageReport) {
	rows := report.Sessions
	truncated := 0
	if len(rows) > usageMaxTableRows {
		truncated = len(rows) - usageMaxTableRows
		rows = rows[:usageMaxTableRows]
	}
	ui.section("sessions (newest first):")
	ui.table(usageTableHeaders(), usageTableRows(rows, report.Totals), usageTableSpec())
	if truncated > 0 {
		fmt.Printf("  ... %d more session(s); use --json for the full list\n", truncated)
	}
}

func usageTableHeaders() []string {
	return []string{"session", "start", "turns", "fresh in", "cache read", "output", "szr cmds", "szr out~", "avoided~", "szr/in", "w/o szr"}
}

func usageTableSpec() tableSpec {
	alignRight := map[int]bool{}
	for column := 2; column <= 10; column++ {
		alignRight[column] = true
	}
	return tableSpec{alignRight: alignRight, maxWidth: map[int]int{0: 12}}
}

func usageTableRows(rows []usageSessionRow, totals usageTotals) [][]string {
	table := make([][]string, 0, len(rows)+1)
	for _, row := range rows {
		table = append(table, usageTableRow(row))
	}
	return append(table, usageTotalsRow(totals))
}

func usageTableRow(row usageSessionRow) []string {
	return []string{
		usageSessionLabel(row),
		row.FirstSeen.Local().Format("2006-01-02 15:04"),
		fmt.Sprintf("%d", row.Turns),
		formatCompactCount(row.FreshInputTokens),
		formatCompactCount(row.CacheReadTokens),
		formatCompactCount(row.OutputTokens),
		fmt.Sprintf("%d", row.SZRCommands),
		formatCompactCount(row.SZREmittedTokens),
		formatCompactCount(row.SZRAvoidedTokens),
		usagePctCell(row.SZRCommands, row.EmittedPct, ""),
		usagePctCell(row.SZRCommands, row.AvoidedPct, "+"),
	}
}

func usageSessionLabel(row usageSessionRow) string {
	label := row.SessionID
	if len(label) > 8 {
		label = label[:8]
	}
	if row.AmbiguousRecords > 0 {
		label += " ?"
	}
	return label
}

func usagePctCell(commands int, pct float64, prefix string) string {
	if commands == 0 {
		return "-"
	}
	return fmt.Sprintf("%s%.1f%%", prefix, pct)
}

func usageTotalsRow(totals usageTotals) []string {
	return []string{
		"total",
		"",
		fmt.Sprintf("%d", totals.Turns),
		formatCompactCount(totals.FreshInputTokens),
		formatCompactCount(totals.CacheReadTokens),
		formatCompactCount(totals.OutputTokens),
		fmt.Sprintf("%d", totals.SZRCommands),
		formatCompactCount(totals.SZREmittedTokens),
		formatCompactCount(totals.SZRAvoidedTokens),
		usagePctCell(totals.SZRCommands, totals.EmittedPct, ""),
		usagePctCell(totals.SZRCommands, totals.AvoidedPct, "+"),
	}
}

func renderUsageNotes(notes []string) {
	fmt.Println()
	for _, note := range notes {
		fmt.Printf("  note: %s\n", note)
	}
}
