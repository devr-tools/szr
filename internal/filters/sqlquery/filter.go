package sqlquery

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var sqlCommandTagPattern = regexp.MustCompile(`^(SELECT|INSERT|UPDATE|DELETE|MERGE|COPY) [0-9]+$`)

func SummarizeSQLQuery(input string, maxLines int) string {
	return summarizeSQLQueryResult(input, maxLines).Text
}

func SQLQueryRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeSQLQueryResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional rows or lines", result.OmittedCount))
}

type sqlQuerySummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeSQLQueryResult(input string, maxLines int) sqlQuerySummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return sqlQuerySummaryResult{Text: "ok"}
	}

	if jsonSummary := summarizeJSONResult(lines, maxLines); jsonSummary.Text != "" {
		return sqlQuerySummaryResult{
			Text:         jsonSummary.Text,
			OmittedCount: jsonSummary.OmittedCount,
		}
	}

	errors, summaries, rows := classifySQLLines(lines)

	if len(errors) > 0 {
		out := append([]string{}, errors...)
		out = append(out, summaries...)
		result := sqlQuerySummaryResult{
			Text: shared.JoinLimitedLines(out, maxLines),
		}
		if len(out) > maxLines {
			result.OmittedCount = len(out) - maxLines
		}
		return result
	}

	if len(rows) == 0 && len(summaries) == 0 {
		return sqlQuerySummaryResult{
			Text: shared.SummarizeGenericFailure(clean, maxLines),
		}
	}

	rawRowCount := len(rows)
	if rawRowCount > maxLines-len(summaries) && len(summaries) == 0 {
		summaries = append(summaries, fmt.Sprintf("rows: %d", rawRowCount))
	}
	kept, omittedRows := selectSQLRows(rows, len(summaries), maxLines)
	out := append([]string{}, kept...)
	if omittedRows > 0 {
		out = append(out, fmt.Sprintf("... +%d more rows", omittedRows))
	}
	out = append(out, summaries...)
	result := sqlQuerySummaryResult{
		Text: strings.Join(out, "\n"),
	}
	if omitted := countSQLOmitted(rawRowCount, len(summaries), maxLines); omitted > 0 {
		result.OmittedCount = omitted
	}
	return result
}

func classifySQLLines(lines []string) ([]string, []string, []string) {
	errors := []string{}
	summaries := []string{}
	rows := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if skipSQLLine(trimmed) {
			continue
		}
		switch classifySQLLine(trimmed) {
		case "error":
			errors = append(errors, shared.Clip(trimmed, 180))
		case "summary":
			summaries = append(summaries, shared.Clip(trimmed, 180))
		default:
			rows = append(rows, shared.Clip(trimmed, 180))
		}
	}
	return normalizeSQLSummarySlice(errors), normalizeSQLSummarySlice(summaries), normalizeSQLSummarySlice(rows)
}

func skipSQLLine(line string) bool {
	return line == "" || isSQLNoise(line) || isSQLBorder(line)
}

func classifySQLLine(line string) string {
	switch {
	case isSQLError(line):
		return "error"
	case isSQLSummary(line):
		return "summary"
	default:
		return "row"
	}
}

func normalizeSQLSummarySlice(lines []string) []string {
	return shared.UniqueStrings(shared.FoldConsecutiveLines(lines))
}

// selectSQLRows keeps every result row when the budget allows, and otherwise
// anomalous rows (rare values in low-cardinality columns such as status or
// state) plus leading rows. The odd row out is usually the reason the query
// was run, so positional truncation must never be what drops it.
func selectSQLRows(rows []string, summaryCount, maxLines int) ([]string, int) {
	limit := maxLines - summaryCount
	if limit < 1 {
		limit = 1
	}
	if len(rows) <= limit {
		return rows, 0
	}
	keep := keepIndices(anomalousSQLRowIndices(rows), len(rows), limit)
	out := filterByIndex(rows, keep)
	return out, len(rows) - len(out)
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

func filterByIndex(items []string, keep map[int]bool) []string {
	out := make([]string, 0, len(keep))
	for i, item := range items {
		if keep[i] {
			out = append(out, item)
		}
	}
	return out
}

const sqlAnomalyMinRows = 8

// anomalousSQLRowIndices reports rows carrying rare values in low-cardinality
// columns, rarest first. Columns are recovered by splitting on the delimiter
// most rows agree on; ID-like columns (nearly all values distinct) never
// qualify, so only status/state/type-shaped columns can flag a row.
func anomalousSQLRowIndices(rows []string) []int {
	if len(rows) < sqlAnomalyMinRows {
		return nil
	}
	cells, columns := splitSQLRows(rows)
	if columns < 2 {
		return nil
	}
	rarity := map[int]int{}
	for col := 0; col < columns; col++ {
		markRareColumnValues(cells, col, len(rows), rarity)
	}
	return sortIndicesByRarity(rarity)
}

func splitSQLRows(rows []string) ([][]string, int) {
	delimiter := pickSQLDelimiter(rows)
	if delimiter == "" {
		return nil, 0
	}
	cells := make([][]string, len(rows))
	for i, row := range rows {
		cells[i] = splitSQLCells(row, delimiter)
	}
	modal, modalCount := modalWidth(cells)
	if modalCount*3 < len(rows)*2 {
		return nil, 0
	}
	return cells, modal
}

func splitSQLCells(row, delimiter string) []string {
	parts := strings.Split(row, delimiter)
	for j, part := range parts {
		parts[j] = strings.TrimSpace(part)
	}
	return parts
}

func modalWidth(cells [][]string) (int, int) {
	counts := map[int]int{}
	for _, row := range cells {
		counts[len(row)]++
	}
	modal, modalCount := 0, 0
	for width, count := range counts {
		if count > modalCount {
			modal, modalCount = width, count
		}
	}
	return modal, modalCount
}

func pickSQLDelimiter(rows []string) string {
	for _, delimiter := range []string{"|", "\t", ","} {
		matching := 0
		for _, row := range rows {
			if strings.Contains(row, delimiter) {
				matching++
			}
		}
		if matching*3 >= len(rows)*2 {
			return delimiter
		}
	}
	return ""
}

// markRareColumnValues records, for each row whose value in the column is
// rare (appears in at most 5% of rows while the column stays low-cardinality),
// how rare that value is.
func markRareColumnValues(cells [][]string, col, rowCount int, rarity map[int]int) {
	counts := columnValueCounts(cells, col)
	if len(counts) < 2 || len(counts) > 8 {
		return
	}
	threshold := rareValueThreshold(rowCount)
	for i, row := range cells {
		if col >= len(row) || row[col] == "" {
			continue
		}
		recordRarity(rarity, i, counts[row[col]], threshold)
	}
}

func columnValueCounts(cells [][]string, col int) map[string]int {
	counts := map[string]int{}
	for _, row := range cells {
		if col < len(row) && row[col] != "" {
			counts[row[col]]++
		}
	}
	return counts
}

func rareValueThreshold(rowCount int) int {
	threshold := rowCount / 20
	if threshold < 1 {
		threshold = 1
	}
	return threshold
}

// recordRarity notes that row idx carries a value seen count times, when
// that is at or below the rare threshold and rarer than anything already
// recorded for the row.
func recordRarity(rarity map[int]int, idx, count, threshold int) {
	if count > threshold {
		return
	}
	if existing, ok := rarity[idx]; !ok || count < existing {
		rarity[idx] = count
	}
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

type sqlJSONSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeJSONResult(lines []string, maxLines int) sqlJSONSummaryResult {
	payload := strings.TrimSpace(strings.Join(lines, "\n"))
	if payload == "" || (!strings.HasPrefix(payload, "[") && !strings.HasPrefix(payload, "{")) {
		return sqlJSONSummaryResult{}
	}

	var decoded any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		return sqlJSONSummaryResult{}
	}

	switch v := decoded.(type) {
	case []any:
		out := []string{fmt.Sprintf("%d row(s)", len(v))}
		limit := maxLines - 1
		if limit < 1 {
			limit = 1
		}
		keep := selectJSONRecordIndices(v, limit)
		for _, idx := range keep {
			encoded, err := json.Marshal(v[idx])
			if err != nil {
				continue
			}
			out = append(out, string(encoded))
		}
		if len(v) > len(keep) {
			out = append(out, fmt.Sprintf("... +%d more rows", len(v)-len(keep)))
		}
		result := sqlJSONSummaryResult{
			Text: strings.Join(out, "\n"),
		}
		if len(v) > len(keep) {
			result.OmittedCount = len(v) - len(keep)
		}
		return result
	case map[string]any:
		encoded, err := json.Marshal(v)
		if err != nil {
			return sqlJSONSummaryResult{}
		}
		return sqlJSONSummaryResult{Text: string(encoded)}
	default:
		return sqlJSONSummaryResult{}
	}
}

// selectJSONRecordIndices keeps up to limit record indices in original order,
// with anomalous records (rare values in low-cardinality string fields)
// always included ahead of positional fill.
func selectJSONRecordIndices(items []any, limit int) []int {
	if len(items) <= limit {
		out := make([]int, len(items))
		for i := range items {
			out[i] = i
		}
		return out
	}
	keep := keepIndices(anomalousJSONRecordIndices(items), len(items), limit)
	out := make([]int, 0, len(keep))
	for i := range items {
		if keep[i] {
			out = append(out, i)
		}
	}
	return out
}

// anomalousJSONRecordIndices applies the low-cardinality rare-value rule to
// the string fields of an array of JSON records.
func anomalousJSONRecordIndices(items []any) []int {
	if len(items) < sqlAnomalyMinRows {
		return nil
	}
	fieldCounts := collectJSONFieldCounts(items)
	threshold := rareValueThreshold(len(items))
	rarity := map[int]int{}
	for i, item := range items {
		markRareJSONRecord(item, fieldCounts, threshold, rarity, i)
	}
	return sortIndicesByRarity(rarity)
}

func collectJSONFieldCounts(items []any) map[string]map[string]int {
	fieldCounts := map[string]map[string]int{}
	for _, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for key, value := range record {
			text := stringFieldValue(value)
			if text == "" {
				continue
			}
			if fieldCounts[key] == nil {
				fieldCounts[key] = map[string]int{}
			}
			fieldCounts[key][text]++
		}
	}
	return fieldCounts
}

func stringFieldValue(value any) string {
	text, _ := value.(string)
	return text
}

func markRareJSONRecord(item any, fieldCounts map[string]map[string]int, threshold int, rarity map[int]int, idx int) {
	record, ok := item.(map[string]any)
	if !ok {
		return
	}
	for key, value := range record {
		text := stringFieldValue(value)
		counts := fieldCounts[key]
		if text == "" || len(counts) < 2 || len(counts) > 8 {
			continue
		}
		recordRarity(rarity, idx, counts[text], threshold)
	}
}

func countSQLOmitted(rawRows, summaryCount, maxLines int) int {
	if maxLines <= 0 {
		return 0
	}
	total := rawRows + summaryCount
	if total <= maxLines {
		return 0
	}
	return total - maxLines
}

func isSQLNoise(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(line, "Pager usage is off."):
		return true
	case strings.HasPrefix(lower, "sqlite version "),
		strings.HasPrefix(lower, "enter \".help\""),
		strings.HasPrefix(lower, "connected to a transient in-memory database"):
		return true
	case strings.HasPrefix(lower, "welcome to the mysql monitor"),
		strings.HasPrefix(lower, "type 'help;'"),
		strings.HasPrefix(lower, "your mysql connection id is "):
		return true
	case strings.HasPrefix(lower, "duckdb "),
		strings.HasPrefix(lower, "enter \".help\" for usage hints"):
		return true
	default:
		return false
	}
}

func isSQLBorder(line string) bool {
	if line == "" {
		return false
	}
	for _, r := range line {
		if !strings.ContainsRune("+-=|:._ \t┌┐└┘├┤┬┴┼─━╔╗╚╝╠╣╦╩╬", r) {
			return false
		}
	}
	return true
}

func isSQLError(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(line, "ERROR:"),
		strings.HasPrefix(line, "Error:"),
		strings.HasPrefix(line, "Parse error"),
		strings.HasPrefix(line, "Runtime error"),
		strings.HasPrefix(line, "LINE "),
		strings.HasPrefix(line, "Catalog Error:"),
		strings.HasPrefix(line, "Parser Error:"),
		strings.HasPrefix(line, "Binder Error:"):
		return true
	case strings.Contains(line, "SQLSTATE["),
		strings.Contains(lower, "syntax error"),
		strings.Contains(lower, "no such table"),
		strings.Contains(lower, "does not exist"),
		strings.Contains(lower, "unknown column"),
		strings.Contains(lower, "unknown database"),
		strings.Contains(lower, "access denied"),
		strings.Contains(lower, "constraint failed"):
		return true
	default:
		return false
	}
}

func isSQLSummary(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case sqlCommandTagPattern.MatchString(line):
		return true
	case strings.HasPrefix(line, "(") && strings.HasSuffix(lower, " row)") ||
		strings.HasPrefix(line, "(") && strings.HasSuffix(lower, " rows)") ||
		strings.HasSuffix(lower, " row in set") ||
		strings.Contains(lower, " rows in set") ||
		strings.HasPrefix(line, "Empty set") ||
		strings.HasPrefix(line, "Query OK"):
		return true
	case strings.HasPrefix(line, "Run Time:"),
		strings.HasPrefix(line, "Time:"):
		return true
	case strings.HasPrefix(line, "CREATE "),
		strings.HasPrefix(line, "ALTER "),
		strings.HasPrefix(line, "DROP "),
		strings.HasPrefix(line, "VACUUM"),
		strings.HasPrefix(line, "ANALYZE"):
		return true
	default:
		return false
	}
}
