package filters

import "fmt"

func ReduceLogLines(lines []string, maxLines int) []string {
	entries, total := collectLogLineEntries(lines)
	if len(entries) == 0 {
		return nil
	}
	maxLines = normalizeReducedLogLimit(maxLines)
	if len(entries) <= maxLines {
		return renderReducedLogLines(entries, total, len(entries))
	}
	return renderReducedLogLines(selectReducedLogEntries(entries, maxLines), total, maxLines)
}

func normalizeReducedLogLimit(maxLines int) int {
	if maxLines <= 0 {
		return 20
	}
	return maxLines
}

func selectReducedLogEntries(entries []logLineEntry, maxLines int) []logLineEntry {
	selected := make(map[string]struct{}, maxLines)
	for _, kind := range []logLineKind{
		logLineKindError,
		logLineKindAnchor,
		logLineKindHint,
		logLineKindDefault,
		logLineKindBoilerplate,
	} {
		selectReducedLogKind(entries, selected, maxLines, kind)
	}
	return orderedReducedLogEntries(entries, selected)
}

func selectReducedLogKind(entries []logLineEntry, selected map[string]struct{}, maxLines int, kind logLineKind) {
	for _, entry := range entries {
		if len(selected) >= maxLines {
			return
		}
		if entry.kind == kind {
			selected[entry.key] = struct{}{}
		}
	}
}

func renderReducedLogLines(entries []logLineEntry, total, maxLines int) []string {
	rendered := make([]string, 0, minInt(len(entries), maxLines)+1)
	keptRaw := 0
	for _, entry := range entries {
		rendered = append(rendered, formatReducedLogLine(entry))
		keptRaw += entry.count
	}
	if omitted := total - keptRaw; omitted > 0 {
		rendered = append(rendered, fmt.Sprintf("... +%d more log lines", omitted))
	}
	return rendered
}

func formatReducedLogLine(entry logLineEntry) string {
	if entry.count <= 1 {
		return entry.line
	}
	return fmt.Sprintf("%s (x%d)", entry.line, entry.count)
}
