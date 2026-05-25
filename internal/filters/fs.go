package filters

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func SummarizeDirectoryListing(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 6
	}

	dirs := []string{}
	files := []string{}
	hidden := 0
	for _, line := range NonEmptyLines(StripANSI(input)) {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "total ") {
			continue
		}
		if strings.HasPrefix(entry, ".") {
			hidden++
		}
		if strings.HasSuffix(entry, "/") {
			dirs = append(dirs, entry)
			continue
		}
		files = append(files, entry)
	}

	dirs = UniqueStrings(dirs)
	files = UniqueStrings(files)
	if len(dirs) == 0 && len(files) == 0 {
		return "empty"
	}
	if len(dirs)+len(files) <= minDirListThreshold(maxLines) {
		entries := append([]string{}, dirs...)
		entries = append(entries, files...)
		return strings.Join(entries, "\n")
	}

	out := []string{}
	if line := summarizeListingGroup("dirs", dirs, 3); line != "" {
		out = append(out, line)
	}
	if line := summarizeListingGroup("files", files, 3); line != "" {
		out = append(out, line)
	}
	if hidden > 0 {
		out = append(out, fmt.Sprintf("hidden: %d", hidden))
	}
	return JoinLimitedLines(out, maxLines)
}

func SummarizeTreeOutput(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}

	root := ""
	top := []*treeSummaryEntry{}
	footer := ""
	var current *treeSummaryEntry
	for _, line := range NonEmptyLines(StripANSI(input)) {
		var entry *treeSummaryEntry
		root, footer, current, entry = consumeTreeSummaryLine(line, root, footer, current)
		if entry != nil {
			top = append(top, entry)
		}
	}

	if root == "" && len(top) == 0 {
		if footer != "" {
			return footer
		}
		return "empty"
	}

	out := []string{}
	if root != "" {
		out = append(out, root)
	}
	for _, entry := range top {
		out = append(out, summarizeTreeEntry(entry))
	}
	if footer != "" && len(out) >= maxLines {
		kept := append([]string{}, out[:maxLines-1]...)
		kept = append(kept, footer)
		return strings.Join(kept, "\n")
	}
	if footer != "" {
		out = append(out, footer)
	}
	return JoinLimitedLines(out, maxLines)
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
		return root, Clip(trimmed, 160), current, nil
	}
	if root == "" && !strings.Contains(line, "-- ") {
		return Clip(trimmed, 160), footer, current, nil
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

func SummarizeReadFile(path string, data []byte, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if json.Valid(data) {
			limit := maxLines
			if limit < 12 {
				limit = 12
			}
			return JoinLimitedLines(NonEmptyLines(RenderJSONStructure(data)), limit)
		}
	case ".md", ".markdown", ".mdx", ".txt", ".rst":
		return summarizeDocPreview(string(data), maxLines)
	}
	return summarizeCodePreview(string(data), maxLines)
}

func summarizeDocPreview(input string, maxLines int) string {
	anchors := []string{}
	sectionLead := []string{}
	seenLead := map[string]struct{}{}
	headingSeen := false
	for _, line := range NonEmptyLines(StripANSI(input)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "#"),
			strings.HasPrefix(trimmed, "- "),
			strings.HasPrefix(trimmed, "* "),
			strings.HasPrefix(trimmed, "1. "),
			strings.HasPrefix(trimmed, "```"),
			strings.HasSuffix(trimmed, ":"):
			anchors = append(anchors, Clip(trimmed, 160))
			headingSeen = strings.HasPrefix(trimmed, "#")
		default:
			if headingSeen {
				key := firstSentence(trimmed)
				if _, ok := seenLead[key]; !ok && key != "" {
					seenLead[key] = struct{}{}
					sectionLead = append(sectionLead, Clip(key, 160))
				}
				headingSeen = false
			}
		}
	}

	return buildPreviewLines(anchors, sectionLead, maxLines)
}

func summarizeCodePreview(input string, maxLines int) string {
	anchors := []string{}
	fallback := []string{}
	for idx, raw := range strings.Split(StripANSI(input), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || isCommentLine(trimmed) {
			continue
		}
		rendered := Clip(normalizePreviewLine(trimmed), 160)
		line := fmt.Sprintf("%4d  %s", idx+1, rendered)
		if isCodeAnchor(trimmed) {
			anchors = append(anchors, line)
			continue
		}
		fallback = append(fallback, line)
	}

	if len(anchors) == 0 && len(fallback) == 0 {
		return ReadLevel([]byte(input), "minimal", true, maxLines)
	}
	return buildPreviewLines(anchors, fallback, maxLines)
}

func buildPreviewLines(anchors, fallback []string, maxLines int) string {
	out := []string{}
	seen := map[string]struct{}{}
	appendUnique := func(values []string) {
		for _, value := range values {
			if len(out) >= maxLines {
				return
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}

	appendUnique(UniqueStrings(anchors))
	appendUnique(UniqueStrings(fallback))
	return JoinLimitedLines(out, maxLines)
}

func normalizePreviewLine(line string) string {
	if strings.Contains(line, "{") && strings.Contains(line, "}") {
		return CollapseBlock(line)
	}
	return line
}

func isCommentLine(line string) bool {
	return strings.HasPrefix(line, "//") ||
		strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, "/*") ||
		strings.HasPrefix(line, "*")
}

func isCodeAnchor(line string) bool {
	for _, prefix := range []string{
		"package ", "import ", "from ", "export ", "type ", "interface ",
		"struct ", "enum ", "class ", "func ", "fn ", "def ", "const ",
		"var ", "let ", "async ", "public ", "private ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.Contains(line, "=>") ||
		strings.Contains(line, "throws") ||
		strings.Contains(line, "TODO") ||
		strings.Contains(line, "FIXME")
}

type treeSummaryEntry struct {
	Name           string
	Children       int
	Descendants    int
	SampleChildren []string
}

func minDirListThreshold(maxLines int) int {
	threshold := maxLines - 1
	if threshold < 4 {
		return 4
	}
	return threshold
}

func summarizeListingGroup(label string, entries []string, limit int) string {
	if len(entries) == 0 {
		return ""
	}
	if limit <= 0 {
		limit = 3
	}
	preview := append([]string{}, entries...)
	sort.Strings(preview)
	if len(preview) > limit {
		preview = append(preview[:limit], fmt.Sprintf("+%d", len(preview)-limit))
	}
	return fmt.Sprintf("%s: %s", label, strings.Join(preview, " "))
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
	return depth, Clip(name, 120)
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

func firstSentence(line string) string {
	line = strings.TrimSpace(line)
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(line, sep); idx >= 0 {
			return line[:idx+1]
		}
	}
	if len([]rune(line)) > 100 {
		return Clip(line, 100)
	}
	return line
}
