package cli

import "fmt"

type usageRenderOptions struct {
	// drillDown appends the single matched session's per-agent table.
	drillDown bool
	// numbered prefixes session rows with pick numbers for the prompt loop.
	numbered bool
}

func renderUsageReport(report usageReport, opts usageRenderOptions) {
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
	renderUsageTable(ui, report, opts.numbered)
	if opts.drillDown {
		renderUsageAgentTable(ui, report.Sessions[0])
	}
	renderUsageNotes(report.Notes)
}

func renderUsageTable(ui spreadUI, report usageReport, numbered bool) {
	rows := report.Sessions
	truncated := 0
	if len(rows) > usageMaxTableRows {
		truncated = len(rows) - usageMaxTableRows
		rows = rows[:usageMaxTableRows]
	}
	ui.section("sessions (newest first):")
	ui.table(usageTableHeaders(numbered), usageTableRows(rows, report.Totals, numbered), usageTableSpec(numbered))
	if truncated > 0 {
		fmt.Printf("  ... %d more session(s); use --json for the full list\n", truncated)
	}
}

func usageTableHeaders(numbered bool) []string {
	headers := []string{"session", "start", "turns", "agents", "fresh in", "cache read", "output", "szr cmds", "szr out~", "avoided~", "szr/in", "w/o szr"}
	if numbered {
		return append([]string{"#"}, headers...)
	}
	return headers
}

func usageTableSpec(numbered bool) tableSpec {
	offset := 0
	if numbered {
		offset = 1
	}
	alignRight := map[int]bool{}
	if numbered {
		alignRight[0] = true
	}
	for column := 2 + offset; column <= 11+offset; column++ {
		alignRight[column] = true
	}
	return tableSpec{alignRight: alignRight, maxWidth: map[int]int{offset: 12}}
}

func usageTableRows(rows []usageSessionRow, totals usageTotals, numbered bool) [][]string {
	table := make([][]string, 0, len(rows)+1)
	for i, row := range rows {
		cells := usageTableRow(row)
		if numbered {
			cells = append([]string{fmt.Sprintf("%d", i+1)}, cells...)
		}
		table = append(table, cells)
	}
	totalsRow := usageTotalsRow(totals)
	if numbered {
		totalsRow = append([]string{""}, totalsRow...)
	}
	return append(table, totalsRow)
}

func usageTableRow(row usageSessionRow) []string {
	return []string{
		usageSessionLabel(row),
		row.FirstSeen.Local().Format("2006-01-02 15:04"),
		fmt.Sprintf("%d", row.Turns),
		fmt.Sprintf("%d", row.AgentCount),
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
		fmt.Sprintf("%d", totals.AgentCount),
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

// renderUsageAgentTable breaks one session down by transcript: one row per
// billed subagent, a main row for the parent transcript's own usage, and the
// session totals. Sessions without subagents render no breakdown.
func renderUsageAgentTable(ui spreadUI, row usageSessionRow) {
	if row.AgentCount == 0 {
		return
	}
	ui.section(fmt.Sprintf("agents in session %s:", usageSessionLabel(row)))
	headers := []string{"agent", "turns", "fresh in", "cache read", "output"}
	spec := tableSpec{alignRight: map[int]bool{1: true, 2: true, 3: true, 4: true}, maxWidth: map[int]int{0: 12}}
	ui.table(headers, usageAgentTableRows(row), spec)
}

func usageAgentTableRows(row usageSessionRow) [][]string {
	table := make([][]string, 0, len(row.Agents)+2)
	for _, agent := range row.Agents {
		table = append(table, usageAgentCells(usageAgentLabel(agent.AgentID), agent.Turns,
			agent.FreshInputTokens, agent.CacheReadTokens, agent.OutputTokens))
	}
	table = append(table, usageAgentCells("main", row.Main.Turns,
		row.Main.FreshInputTokens(), row.Main.CacheReadTokens, row.Main.OutputTokens))
	return append(table, usageAgentCells("total", row.Turns,
		row.FreshInputTokens, row.CacheReadTokens, row.OutputTokens))
}

func usageAgentCells(label string, turns, freshIn, cacheRead, output int) []string {
	return []string{
		label,
		fmt.Sprintf("%d", turns),
		formatCompactCount(freshIn),
		formatCompactCount(cacheRead),
		formatCompactCount(output),
	}
}

func usageAgentLabel(id string) string {
	if len(id) > 8 {
		id = id[:8]
	}
	return id
}

func renderUsageNotes(notes []string) {
	fmt.Println()
	for _, note := range notes {
		fmt.Printf("  note: %s\n", note)
	}
}
