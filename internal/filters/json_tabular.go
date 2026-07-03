package filters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Tabular JSON preview: arrays of uniform objects (the dominant shape of API
// list responses) render as one header line that declares the keys once plus
// |-delimited value rows. Compared to repeating key=value per item this
// conveys the same values in roughly half the tokens, and the row budget is
// spent on anomalous rows (minority values, non-uniform items) before leading
// rows, so the odd item out is never what positional truncation drops.
const (
	// tabularJSONMinItems is the smallest array worth a table; below it the
	// header overhead outweighs the per-row savings.
	tabularJSONMinItems = 5
	// tabularJSONMinSharePct is the minimum share of items that must carry the
	// dominant key set for the array to count as uniform.
	tabularJSONMinSharePct = 80
	// tabularJSONMaxCols caps the columns shown per row; extra columns are
	// declared in the header as "+N more cols" instead of bloating every row.
	tabularJSONMaxCols = 10
	// tabularJSONCellClip bounds a single cell's rendered width.
	tabularJSONCellClip = 64
	// tabularJSONAnomalyMinRows gates rarity detection: tiny tables are shown
	// in full anyway, so rarity scoring only runs above this row count.
	tabularJSONAnomalyMinRows = 8
)

// SummarizeUniformJSONArray renders items as a tabular JSON preview when the
// array is uniform enough (>=tabularJSONMinItems items, >=80% sharing one key
// set with >=2 keys). label names the container in the header ("array" for
// JSON arrays, "stream" for NDJSON). Returns false when the shape does not
// qualify; callers keep their existing rendering in that case.
func SummarizeUniformJSONArray(label string, items []any, maxLines int) (string, bool) {
	if maxLines <= 0 {
		maxLines = 12
	}
	cols, ok := dominantJSONKeySet(items)
	if !ok {
		return "", false
	}

	shownCols := cols
	if len(shownCols) > tabularJSONMaxCols {
		shownCols = shownCols[:tabularJSONMaxCols]
	}
	rows := buildTabularJSONRows(items, cols, shownCols)
	kept, omitted := selectTabularJSONRows(rows, maxLines)
	out := append([]string{tabularJSONHeader(label, len(items), cols, shownCols)}, kept...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d more rows", omitted))
	}
	return strings.Join(out, "\n"), true
}

func tabularJSONHeader(label string, total int, cols, shownCols []string) string {
	if label == "" {
		label = "array"
	}
	header := fmt.Sprintf("%s len=%d uniform objects, cols: %s", label, total, strings.Join(quoteJSONCols(shownCols), "|"))
	if extra := len(cols) - len(shownCols); extra > 0 {
		header += fmt.Sprintf(" (+%d more cols)", extra)
	}
	return header
}

type tabularJSONRow struct {
	line       string
	conforming bool
	cells      []string
}

// dominantJSONKeySet reports the priority-ordered key set shared by at least
// tabularJSONMinSharePct of items, or false when no such set exists.
func dominantJSONKeySet(items []any) ([]string, bool) {
	if len(items) < tabularJSONMinItems {
		return nil, false
	}
	bestID, bestCount := dominantJSONKeySetID(items)
	if bestID == "" || bestCount*100 < len(items)*tabularJSONMinSharePct {
		return nil, false
	}
	cols := strings.Split(bestID, "\x00")
	if len(cols) < 2 {
		return nil, false
	}
	sortJSONColsByPriority(cols)
	return cols, true
}

func dominantJSONKeySetID(items []any) (string, int) {
	counts := map[string]int{}
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			counts[jsonKeySetID(obj)]++
		}
	}
	bestID, bestCount := "", 0
	for id, count := range counts {
		if count > bestCount || (count == bestCount && id < bestID) {
			bestID, bestCount = id, count
		}
	}
	return bestID, bestCount
}

func sortJSONColsByPriority(cols []string) {
	sort.SliceStable(cols, func(i, j int) bool {
		pi, pj := jsonKeyPriority(cols[i]), jsonKeyPriority(cols[j])
		if pi != pj {
			return pi < pj
		}
		return cols[i] < cols[j]
	})
}

func jsonKeySetID(obj map[string]any) string {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\x00")
}

// buildTabularJSONRows renders one row per item: cells for items matching the
// dominant key set (cols), inline previews for the rest. shownCols may be a
// capped prefix of cols.
func buildTabularJSONRows(items []any, cols, shownCols []string) []tabularJSONRow {
	dominantID := jsonColsSetID(cols)
	rows := make([]tabularJSONRow, len(items))
	for i, item := range items {
		rows[i] = buildTabularJSONRow(i, item, dominantID, shownCols)
	}
	return rows
}

func jsonColsSetID(cols []string) string {
	sorted := append([]string(nil), cols...)
	sort.Strings(sorted)
	return strings.Join(sorted, "\x00")
}

func buildTabularJSONRow(index int, item any, dominantID string, shownCols []string) tabularJSONRow {
	obj, isObj := item.(map[string]any)
	if !isObj || jsonKeySetID(obj) != dominantID {
		// Non-uniform items are the payload in a uniform list, so they always
		// carry their index and a compact key=value preview.
		return tabularJSONRow{line: fmt.Sprintf("#%d %s", index, inlineJSONValuePreview(item))}
	}
	cells := make([]string, len(shownCols))
	for c, col := range shownCols {
		cells[c] = tabularJSONCell(obj[col])
	}
	return tabularJSONRow{line: strings.Join(cells, "|"), conforming: true, cells: cells}
}

// selectTabularJSONRows keeps every row when the budget allows, and otherwise
// anomalous rows (non-uniform items first, then rare values in
// low-cardinality columns) plus leading rows. Rows emitted after a gap carry
// a "#index" prefix so position stays recoverable.
func selectTabularJSONRows(rows []tabularJSONRow, maxLines int) ([]string, int) {
	keep := keptTabularJSONRowIndices(rows, maxLines)
	out := make([]string, 0, len(keep))
	prev := -1
	for i, row := range rows {
		if !keep[i] {
			continue
		}
		line := row.line
		if prev != i-1 && row.conforming {
			line = fmt.Sprintf("#%d %s", i, line)
		}
		out = append(out, line)
		prev = i
	}
	return out, len(rows) - len(keep)
}

func keptTabularJSONRowIndices(rows []tabularJSONRow, maxLines int) map[int]bool {
	keep := map[int]bool{}
	if len(rows) <= maxLines-1 {
		for i := range rows {
			keep[i] = true
		}
		return keep
	}
	limit := maxLines - 2
	if limit < 3 {
		limit = 3
	}
	markKeptJSONRows(keep, nonConformingJSONRowIndices(rows), limit)
	markKeptJSONRows(keep, anomalousJSONRowIndices(rows), limit)
	for i := 0; i < len(rows) && len(keep) < limit; i++ {
		keep[i] = true
	}
	return keep
}

func markKeptJSONRows(keep map[int]bool, indices []int, limit int) {
	for _, idx := range indices {
		if len(keep) >= limit {
			return
		}
		keep[idx] = true
	}
}

func nonConformingJSONRowIndices(rows []tabularJSONRow) []int {
	out := []int{}
	for i, row := range rows {
		if !row.conforming {
			out = append(out, i)
		}
	}
	return out
}

// anomalousJSONRowIndices reports conforming rows carrying rare values (at
// most 5% of rows) in low-cardinality columns, rarest first — the same
// heuristic the wide-table filter uses, so ID-, title-, and timestamp-shaped
// columns are naturally skipped because nearly every value is distinct.
func anomalousJSONRowIndices(rows []tabularJSONRow) []int {
	conforming, width := conformingJSONRowShape(rows)
	if conforming < tabularJSONAnomalyMinRows {
		return nil
	}
	threshold := conforming / 20
	if threshold < 1 {
		threshold = 1
	}
	rarity := map[int]int{}
	for col := 0; col < width; col++ {
		markRareJSONColumn(rows, col, threshold, rarity)
	}
	return sortJSONIndicesByRarity(rarity)
}

func conformingJSONRowShape(rows []tabularJSONRow) (int, int) {
	conforming, width := 0, 0
	for _, row := range rows {
		if row.conforming {
			conforming++
			width = len(row.cells)
		}
	}
	return conforming, width
}

func sortJSONIndicesByRarity(rarity map[int]int) []int {
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

func markRareJSONColumn(rows []tabularJSONRow, col, threshold int, rarity map[int]int) {
	counts := map[string]int{}
	for _, row := range rows {
		if row.conforming {
			counts[row.cells[col]]++
		}
	}
	if len(counts) < 2 || len(counts) > 8 {
		return
	}
	for i, row := range rows {
		if !row.conforming {
			continue
		}
		recordJSONRowRarity(rarity, i, counts[row.cells[col]], threshold)
	}
}

func recordJSONRowRarity(rarity map[int]int, idx, count, threshold int) {
	if count > threshold {
		return
	}
	if existing, ok := rarity[idx]; !ok || count < existing {
		rarity[idx] = count
	}
}

// tabularJSONCell renders one value for a row. Strings that could be misread
// (containing the delimiter, quotes, or whitespace edges, or spelling a JSON
// literal or number) are quoted so bare null/true/false/123 stay
// unambiguously non-string; nested containers are summarized, never
// fake-flattened.
func tabularJSONCell(value any) string {
	switch node := value.(type) {
	case string:
		clipped := Clip(node, tabularJSONCellClip)
		if tabularJSONCellNeedsQuoting(clipped) {
			return strconv.Quote(clipped)
		}
		return clipped
	case map[string]any:
		return tabularJSONObjectCell(node)
	case []any:
		return fmt.Sprintf("[%d items]", len(node))
	default:
		return previewScalar(node)
	}
}

func tabularJSONObjectCell(node map[string]any) string {
	if len(node) == 0 {
		return "{}"
	}
	entries := sortedJSONKeys(node)
	shown := entries
	if len(shown) > 3 {
		shown = shown[:3]
	}
	names := make([]string, 0, len(shown)+1)
	for _, entry := range shown {
		names = append(names, entry.key)
	}
	if extra := len(entries) - len(shown); extra > 0 {
		names = append(names, fmt.Sprintf("+%d", extra))
	}
	return "{" + strings.Join(names, ",") + "}"
}

func tabularJSONCellNeedsQuoting(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return true
	}
	if strings.ContainsAny(value, "|\"\n\r\t\\") {
		return true
	}
	switch value {
	case "null", "true", "false":
		return true
	}
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return true
	}
	return false
}

func quoteJSONCols(cols []string) []string {
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		if strings.ContainsAny(col, "|\" \n\r\t") {
			col = strconv.Quote(col)
		}
		out = append(out, col)
	}
	return out
}

// inlineJSONValuePreview folds a non-uniform array item into a single
// key=value line, priority keys first.
func inlineJSONValuePreview(value any) string {
	obj, ok := value.(map[string]any)
	if !ok {
		return previewArrayElement(value)
	}
	entries := sortedJSONKeys(obj)
	shown := entries
	if len(shown) > 6 {
		shown = shown[:6]
	}
	parts := make([]string, 0, len(shown)+1)
	for _, entry := range shown {
		parts = append(parts, entry.key+"="+inlineJSONCell(obj[entry.key]))
	}
	if extra := len(entries) - len(shown); extra > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", extra))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// inlineJSONCell is tabularJSONCell for space-delimited inline previews:
// strings always quote, because a bare string with spaces would blur into
// the next key=value pair.
func inlineJSONCell(value any) string {
	if _, ok := value.(string); ok {
		return previewScalar(value)
	}
	return tabularJSONCell(value)
}
