package filters

import (
	"fmt"
	"sort"
	"strings"
)

type logLineKind uint8

const (
	logLineKindDefault logLineKind = iota
	logLineKindBoilerplate
	logLineKindHint
	logLineKindAnchor
	logLineKindError
)

type logLineEntry struct {
	key       string
	line      string
	firstSeen int
	lastSeen  int
	count     int
	kind      logLineKind
}

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

func orderedReducedLogEntries(entries []logLineEntry, selected map[string]struct{}) []logLineEntry {
	ordered := make([]logLineEntry, 0, len(selected))
	for _, entry := range entries {
		if _, ok := selected[entry.key]; ok {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

//nolint:maintidx // Entry collection intentionally combines dedupe, ordering, and signal promotion in one pass.
func collectLogLineEntries(lines []string) ([]logLineEntry, int) {
	indexByKey := map[string]int{}
	entries := make([]logLineEntry, 0, len(lines))
	total := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		total++

		key := reduceLogLineKey(line)
		if key == "" {
			continue
		}
		kind := classifyLogLine(line)
		if idx, ok := indexByKey[key]; ok {
			entries[idx].count++
			entries[idx].lastSeen = total - 1
			entries[idx].kind = maxLogLineKind(entries[idx].kind, kind)
			continue
		}

		indexByKey[key] = len(entries)
		entries = append(entries, logLineEntry{
			key:       key,
			line:      line,
			firstSeen: total - 1,
			lastSeen:  total - 1,
			count:     1,
			kind:      kind,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].firstSeen != entries[j].firstSeen {
			return entries[i].firstSeen < entries[j].firstSeen
		}
		return entries[i].lastSeen < entries[j].lastSeen
	})
	return entries, total
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

func reduceLogLineKey(line string) string {
	line = similarLineKey(line)
	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}

func classifyLogLine(line string) logLineKind {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case lower == "":
		return logLineKindBoilerplate
	case isErrorLogLine(lower):
		return logLineKindError
	case isAnchorLogLine(line, lower):
		return logLineKindAnchor
	case isHintLogLine(lower):
		return logLineKindHint
	case isBoilerplateLogLine(lower):
		return logLineKindBoilerplate
	default:
		return logLineKindDefault
	}
}

func isErrorLogLine(lower string) bool {
	for _, marker := range []string{
		"panic:",
		"fatal",
		"error",
		"failed",
		"failure",
		"exception",
		"traceback",
		"assertionerror",
		"segmentation fault",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isAnchorLogLine(line, lower string) bool {
	if DiagnosticAnchor(line) != "" {
		return true
	}
	if strings.HasPrefix(lower, "at ") || strings.HasPrefix(lower, "#") {
		return true
	}
	return strings.Contains(lower, "caused by:")
}

func isHintLogLine(lower string) bool {
	for _, marker := range []string{"warning", "warn:", "hint:", "help:", "note:"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isBoilerplateLogLine(lower string) bool {
	for _, marker := range []string{
		"debug",
		"info",
		"trace",
		"progress",
		"downloading",
		"downloaded",
		"fetching",
		"installing",
		"installed",
		"resolving",
		"waiting",
		"retrying",
		"connected",
		"heartbeat",
		"listening on",
		"watching for changes",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func maxLogLineKind(left, right logLineKind) logLineKind {
	if right > left {
		return right
	}
	return left
}
