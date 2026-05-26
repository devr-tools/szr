package fs

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

func SummarizeTreeOutput(input string, maxLines int) string {
	return summarizeTreeOutputResult(input, maxLines).Text
}

func TreeOutputRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeTreeOutputResult(input, maxLines)
	if !result.Omitted {
		return filters.NoRecovery()
	}
	return filters.FullOutputRecovery("omitted tree entries")
}

func summarizeTreeOutputResult(input string, maxLines int) treeOutputSummaryResult {
	if maxLines <= 0 {
		maxLines = 8
	}

	root, top, footer := collectTreeSummaryEntries(input)
	if root == "" && len(top) == 0 {
		if footer != "" {
			return treeOutputSummaryResult{Text: footer}
		}
		return treeOutputSummaryResult{Text: "empty"}
	}

	out := renderTreeSummaryLines(root, top, footer)
	omitted := treeSummaryEntriesOmitted(top) || len(out) > maxLines
	if footer != "" && len(out) >= maxLines {
		return summarizeTreeWithFooter(out, maxLines)
	}
	return treeOutputSummaryResult{
		Text:    filters.JoinLimitedLines(out, maxLines),
		Omitted: omitted,
	}
}

func collectTreeSummaryEntries(input string) (string, []*treeSummaryEntry, string) {
	root := ""
	top := []*treeSummaryEntry{}
	footer := ""
	var current *treeSummaryEntry
	for _, line := range filters.NonEmptyLines(filters.StripANSI(input)) {
		var entry *treeSummaryEntry
		root, footer, current, entry = consumeTreeSummaryLine(line, root, footer, current)
		if entry != nil {
			top = append(top, entry)
		}
	}
	return root, top, footer
}

func renderTreeSummaryLines(root string, top []*treeSummaryEntry, footer string) []string {
	out := []string{}
	if root != "" {
		out = append(out, root)
	}
	for _, entry := range top {
		out = append(out, summarizeTreeEntry(entry))
	}
	if footer != "" {
		out = append(out, footer)
	}
	return out
}

func treeSummaryEntriesOmitted(top []*treeSummaryEntry) bool {
	for _, entry := range top {
		if entry.Children > len(entry.SampleChildren) || entry.Descendants > 0 {
			return true
		}
	}
	return false
}

func summarizeTreeWithFooter(out []string, maxLines int) treeOutputSummaryResult {
	kept := append([]string{}, out[:maxLines-1]...)
	kept = append(kept, out[len(out)-1])
	return treeOutputSummaryResult{
		Text:    strings.Join(kept, "\n"),
		Omitted: true,
	}
}

func consumeTreeSummaryLine(
	line string,
	root string,
	footer string,
	current *treeSummaryEntry,
) (string, string, *treeSummaryEntry, *treeSummaryEntry) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return root, footer, current, nil
	}
	if strings.Contains(trimmed, " director") || strings.Contains(trimmed, " file") {
		return root, filters.Clip(trimmed, 160), current, nil
	}
	if root == "" && !strings.Contains(line, "-- ") {
		return filters.Clip(trimmed, 160), footer, current, nil
	}

	depth, name := parseTreeLine(line)
	if name == "" {
		return root, footer, current, nil
	}
	nextCurrent, entry := updateTreeSummaryEntry(current, depth, name)
	return root, footer, nextCurrent, entry
}

func updateTreeSummaryEntry(
	current *treeSummaryEntry,
	depth int,
	name string,
) (*treeSummaryEntry, *treeSummaryEntry) {
	switch depth {
	case 1:
		entry := &treeSummaryEntry{Name: name}
		return entry, entry
	case 2:
		if current != nil {
			current.Children++
			if len(current.SampleChildren) < 2 {
				current.SampleChildren = append(current.SampleChildren, name)
			}
		}
	default:
		if current != nil {
			current.Descendants++
		}
	}
	return current, nil
}

type treeSummaryEntry struct {
	Name           string
	Children       int
	Descendants    int
	SampleChildren []string
}

type treeOutputSummaryResult struct {
	Text    string
	Omitted bool
}

func parseTreeLine(line string) (int, string) {
	idx := strings.Index(line, "-- ")
	if idx < 0 {
		return 0, ""
	}
	prefix := line[:idx]
	depth := 1
	for i := 0; i+3 < len(prefix); i += 4 {
		chunk := prefix[i : i+4]
		if chunk == "|   " || chunk == "    " {
			depth++
		}
	}
	name := strings.TrimSpace(line[idx+3:])
	return depth, filters.Clip(name, 120)
}

func summarizeTreeEntry(entry *treeSummaryEntry) string {
	if entry == nil {
		return ""
	}
	label := entry.Name
	if entry.Children == 0 && entry.Descendants == 0 {
		return label
	}
	parts := []string{label}
	if entry.Children > 0 {
		parts = append(parts, fmt.Sprintf("(%d)", entry.Children))
	}
	if len(entry.SampleChildren) > 0 {
		parts = append(parts, strings.Join(entry.SampleChildren, ", "))
	}
	if entry.Children > len(entry.SampleChildren) {
		parts = append(parts, fmt.Sprintf("+%d", entry.Children-len(entry.SampleChildren)))
	}
	return strings.Join(parts, " ")
}
