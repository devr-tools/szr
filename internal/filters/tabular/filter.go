package tabular

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var wideColumnSplit = regexp.MustCompile(`\t+|\s{2,}`)

type parsedTable struct {
	headers []string
	rows    [][]string
}

func SummarizeWideTable(input string, maxLines int) string {
	return summarizeWideTableResult(input, maxLines).Text
}

func WideTableRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeWideTableResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional rows", result.OmittedCount))
}

type tabularSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeWideTableResult(input string, maxLines int) tabularSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return tabularSummaryResult{Text: "ok"}
	}

	if table, ok := parseDU(lines); ok {
		return renderTableSummary(table, maxLines)
	}
	if table, ok := parseWideTable(lines); ok {
		return renderTableSummary(table, maxLines)
	}
	return tabularSummaryResult{Text: shared.CompactLines(clean, maxLines)}
}

func parseDU(lines []string) (parsedTable, bool) {
	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			return parsedTable{}, false
		}
		size := fields[0]
		if !looksLikeSize(size) {
			return parsedTable{}, false
		}
		rows = append(rows, []string{size, strings.Join(fields[1:], " ")})
	}
	if len(rows) == 0 {
		return parsedTable{}, false
	}
	return parsedTable{
		headers: []string{"SIZE", "PATH"},
		rows:    rows,
	}, true
}

func parseWideTable(lines []string) (parsedTable, bool) {
	headers := splitWideRow(lines[0])
	if fallback := strings.Fields(strings.TrimSpace(lines[0])); len(fallback) > len(headers) {
		headers = fallback
	}
	if len(headers) < 2 {
		return parsedTable{}, false
	}

	rows := make([][]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		row := splitWideRow(line)
		if fallback := strings.Fields(strings.TrimSpace(line)); len(row) < len(headers) && len(fallback) >= 2 {
			row = fallback
		}
		if len(row) < 2 {
			continue
		}
		row = alignRow(row, len(headers))
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return parsedTable{}, false
	}

	return parsedTable{
		headers: normalizeHeaders(headers),
		rows:    rows,
	}, true
}

func splitWideRow(line string) []string {
	parts := wideColumnSplit.Split(strings.TrimSpace(line), -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func alignRow(row []string, width int) []string {
	switch {
	case len(row) == width:
		return row
	case len(row) > width:
		out := append([]string{}, row[:width-1]...)
		out = append(out, strings.Join(row[width-1:], " "))
		return out
	default:
		out := append([]string{}, row...)
		for len(out) < width {
			out = append(out, "")
		}
		return out
	}
}

func normalizeHeaders(headers []string) []string {
	out := make([]string, 0, len(headers))
	for _, header := range headers {
		header = strings.TrimSpace(header)
		header = strings.Trim(header, ":")
		header = strings.ReplaceAll(header, "\t", " ")
		header = strings.Join(strings.Fields(header), " ")
		header = strings.ToUpper(header)
		out = append(out, header)
	}
	return out
}

func renderTableSummary(table parsedTable, maxLines int) tabularSummaryResult {
	header := fmt.Sprintf("rows: %d columns: %s", len(table.rows), strings.Join(table.headers, ", "))
	kept, omitted := selectTableRows(table, maxLines)
	lines := append([]string{header}, kept...)
	if omitted > 0 {
		lines = append(lines, fmt.Sprintf("... +%d more rows", omitted))
	}
	return tabularSummaryResult{
		Text:         strings.Join(lines, "\n"),
		OmittedCount: omitted,
	}
}

// selectTableRows keeps every row when the budget allows, and otherwise
// anomalous rows (rare values in low-cardinality columns such as STATUS or
// STATE) plus leading rows. In wide tables the odd row out is the payload, so
// positional truncation must never be what drops it.
func selectTableRows(table parsedTable, maxLines int) ([]string, int) {
	limit := maxLines - 1
	if limit < 1 {
		limit = 1
	}
	if len(table.rows) <= limit {
		return summarizeRows(table, nil), 0
	}
	keep := keepIndices(anomalousTableRowIndices(table.rows), len(table.rows), limit)
	return summarizeRows(table, keep), len(table.rows) - len(keep)
}

// keepIndices marks up to limit indices as kept: the anomalous ones first,
// then leading indices as positional fill.
func keepIndices(anomalies []int, total, limit int) map[int]bool {
	keep := map[int]bool{}
	for _, idx := range anomalies {
		if len(keep) >= limit {
			break
		}
		keep[idx] = true
	}
	for i := 0; i < total && len(keep) < limit; i++ {
		keep[i] = true
	}
	return keep
}

func summarizeRows(table parsedTable, keep map[int]bool) []string {
	out := make([]string, 0, len(table.rows))
	for i, row := range table.rows {
		if keep != nil && !keep[i] {
			continue
		}
		out = append(out, shared.Clip(summarizeRow(table.headers, row), 160))
	}
	return out
}

const tableAnomalyMinRows = 8

// anomalousTableRowIndices reports rows carrying rare values (at most 5% of
// rows) in low-cardinality columns, rarest first. ID-, name-, and
// timestamp-shaped columns are naturally skipped because nearly every value
// is distinct.
func anomalousTableRowIndices(rows [][]string) []int {
	if len(rows) < tableAnomalyMinRows {
		return nil
	}
	threshold := len(rows) / 20
	if threshold < 1 {
		threshold = 1
	}
	rarity := map[int]int{}
	for col := 0; col < len(rows[0]); col++ {
		markRareTableColumn(rows, col, threshold, rarity)
	}
	return sortIndicesByRarity(rarity)
}

func sortIndicesByRarity(rarity map[int]int) []int {
	out := make([]int, 0, len(rarity))
	for idx := range rarity {
		out = append(out, idx)
	}
	sort.Slice(out, func(a, b int) bool {
		if rarity[out[a]] == rarity[out[b]] {
			return out[a] < out[b]
		}
		return rarity[out[a]] < rarity[out[b]]
	})
	return out
}

func markRareTableColumn(rows [][]string, col, threshold int, rarity map[int]int) {
	counts := tableColumnCounts(rows, col)
	if len(counts) < 2 || len(counts) > 8 {
		return
	}
	for i, row := range rows {
		if col >= len(row) || row[col] == "" {
			continue
		}
		recordTableRarity(rarity, i, counts[row[col]], threshold)
	}
}

func tableColumnCounts(rows [][]string, col int) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		if col < len(row) && row[col] != "" {
			counts[row[col]]++
		}
	}
	return counts
}

func recordTableRarity(rarity map[int]int, idx, count, threshold int) {
	if count > threshold {
		return
	}
	if existing, ok := rarity[idx]; !ok || count < existing {
		rarity[idx] = count
	}
}

func summarizeRow(headers []string, row []string) string {
	keyIndex := pickKeyColumn(headers)
	keyLabel := normalizeLabel(headers[keyIndex])
	keyValue := strings.TrimSpace(row[keyIndex])
	if keyValue == "" {
		keyValue = "unknown"
	}

	detailIndexes := pickDetailColumns(headers, keyIndex)
	details := make([]string, 0, len(detailIndexes))
	for _, idx := range detailIndexes {
		value := strings.TrimSpace(row[idx])
		if value == "" {
			continue
		}
		details = append(details, fmt.Sprintf("%s=%s", normalizeLabel(headers[idx]), value))
	}
	if len(details) == 0 {
		return fmt.Sprintf("%s=%s", keyLabel, keyValue)
	}
	return fmt.Sprintf("%s=%s %s", keyLabel, keyValue, strings.Join(details, " "))
}

func pickKeyColumn(headers []string) int {
	priority := []string{"NAME", "NAMES", "UNIT", "FILESYSTEM", "PATH", "MOUNTED ON", "RELEASE", "PID"}
	for _, want := range priority {
		for idx, header := range headers {
			if header == want {
				return idx
			}
		}
	}
	return 0
}

func pickDetailColumns(headers []string, keyIndex int) []int {
	type candidate struct {
		header string
		index  int
	}

	byName := map[string]int{}
	for idx, header := range headers {
		byName[header] = idx
	}

	priority := []string{
		"READY", "STATUS", "STATE", "ACTIVE", "SUB",
		"USE%", "SIZE", "USED", "AVAIL", "AVAILABLE",
		"CPU%", "%CPU", "MEM%", "%MEM", "ELAPSED", "ETIME", "AGE",
		"IP", "NODE", "IMAGE", "COMMAND", "DESCRIPTION",
	}

	chosen := make([]candidate, 0, 4)
	seen := map[int]struct{}{}
	for _, name := range priority {
		idx, ok := byName[name]
		if !ok || idx == keyIndex {
			continue
		}
		seen[idx] = struct{}{}
		chosen = append(chosen, candidate{header: name, index: idx})
		if len(chosen) >= 4 {
			break
		}
	}

	if len(chosen) == 0 {
		for idx := range headers {
			if idx == keyIndex {
				continue
			}
			chosen = append(chosen, candidate{header: headers[idx], index: idx})
			if len(chosen) >= 4 {
				break
			}
		}
	}

	out := make([]int, 0, len(chosen))
	for _, item := range chosen {
		out = append(out, item.index)
	}
	return out
}

func normalizeLabel(header string) string {
	label := strings.ToLower(strings.TrimSpace(header))
	label = strings.ReplaceAll(label, " ", "_")
	label = strings.TrimPrefix(label, "%")
	label = strings.TrimSuffix(label, "%")
	label = strings.ReplaceAll(label, "%", "pct")
	return label
}

func looksLikeSize(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || r == '.' {
			continue
		}
		switch r {
		case 'K', 'M', 'G', 'T', 'P', 'B', 'i', 'k', 'm', 'g', 't', 'p':
			continue
		default:
			return false
		}
	}
	return true
}
