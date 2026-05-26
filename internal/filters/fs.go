package filters

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func SummarizeDirectoryListing(input string, maxLines int) string {
	return summarizeDirectoryListingResult(input, maxLines).Text
}

func DirectoryListingRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDirectoryListingResult(input, maxLines)
	if !result.Grouped || result.EntryCount == 0 {
		return NoRecovery()
	}
	return FullOutputRecovery(fmt.Sprintf("omitted %d directory entries", result.EntryCount))
}

func summarizeDirectoryListingResult(input string, maxLines int) listingSummaryResult {
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
		return listingSummaryResult{Text: "empty"}
	}
	entryCount := len(dirs) + len(files)
	if entryCount <= minDirListThreshold(maxLines) {
		entries := append([]string{}, dirs...)
		entries = append(entries, files...)
		return listingSummaryResult{
			Text:       strings.Join(entries, "\n"),
			EntryCount: entryCount,
		}
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
	return listingSummaryResult{
		Text:       JoinLimitedLines(out, maxLines),
		EntryCount: entryCount,
		Grouped:    true,
	}
}

func SummarizeTreeOutput(input string, maxLines int) string {
	return summarizeTreeOutputResult(input, maxLines).Text
}

func TreeOutputRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeTreeOutputResult(input, maxLines)
	if !result.Omitted {
		return NoRecovery()
	}
	return FullOutputRecovery("omitted tree entries")
}

func summarizeTreeOutputResult(input string, maxLines int) treeOutputSummaryResult {
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
			return treeOutputSummaryResult{Text: footer}
		}
		return treeOutputSummaryResult{Text: "empty"}
	}

	out := []string{}
	if root != "" {
		out = append(out, root)
	}
	for _, entry := range top {
		out = append(out, summarizeTreeEntry(entry))
	}
	omitted := false
	for _, entry := range top {
		if entry.Children > len(entry.SampleChildren) || entry.Descendants > 0 {
			omitted = true
			break
		}
	}
	if footer != "" && len(out) >= maxLines {
		kept := append([]string{}, out[:maxLines-1]...)
		kept = append(kept, footer)
		return treeOutputSummaryResult{
			Text:    strings.Join(kept, "\n"),
			Omitted: true,
		}
	}
	if footer != "" {
		out = append(out, footer)
	}
	if len(out) > maxLines {
		omitted = true
	}
	return treeOutputSummaryResult{
		Text:    JoinLimitedLines(out, maxLines),
		Omitted: omitted,
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
	return summarizeReadFileResult(path, data, maxLines).Text
}

func ReadFileRecoveryInfo(path string, data []byte, maxLines int) (string, string, bool) {
	result := summarizeReadFileResult(path, data, maxLines)
	if result.RawLineCount == 0 || result.PreviewLineCount >= result.RawLineCount {
		return NoRecovery()
	}
	return FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.RawLineCount-result.PreviewLineCount))
}

func summarizeReadFileResult(path string, data []byte, maxLines int) readFileSummaryResult {
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
			lines := NonEmptyLines(RenderJSONStructure(data))
			return readFileSummaryResult{
				Text:             JoinLimitedLines(lines, limit),
				RawLineCount:     len(lines),
				PreviewLineCount: minInt(len(lines), limit),
			}
		}
	case ".md", ".markdown", ".mdx", ".txt", ".rst":
		return summarizeDocPreviewResult(string(data), maxLines)
	}
	return summarizeCodePreviewResult(string(data), maxLines)
}

func summarizeDocPreview(input string, maxLines int) string {
	return summarizeDocPreviewResult(input, maxLines).Text
}

func summarizeDocPreviewResult(input string, maxLines int) readFileSummaryResult {
	anchors := []string{}
	sectionLead := []string{}
	seenLead := map[string]struct{}{}
	headingSeen := false
	rawLineCount := 0
	for _, line := range NonEmptyLines(StripANSI(input)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		rawLineCount++
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

	text, previewLineCount := buildPreviewLines(anchors, sectionLead, maxLines)
	return readFileSummaryResult{
		Text:             text,
		RawLineCount:     rawLineCount,
		PreviewLineCount: previewLineCount,
	}
}

func summarizeCodePreview(input string, maxLines int) string {
	return summarizeCodePreviewResult(input, maxLines).Text
}

func summarizeCodePreviewResult(input string, maxLines int) readFileSummaryResult {
	anchors := []string{}
	fallback := []string{}
	rawLineCount := 0
	for idx, raw := range strings.Split(StripANSI(input), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || isCommentLine(trimmed) {
			continue
		}
		rawLineCount++
		rendered := Clip(normalizePreviewLine(trimmed), 160)
		line := fmt.Sprintf("%4d  %s", idx+1, rendered)
		if isCodeAnchor(trimmed) {
			anchors = append(anchors, line)
			continue
		}
		fallback = append(fallback, line)
	}

	if len(anchors) == 0 && len(fallback) == 0 {
		text := ReadLevel([]byte(input), "minimal", true, maxLines)
		return readFileSummaryResult{
			Text:             text,
			RawLineCount:     len(NonEmptyLines(StripANSI(input))),
			PreviewLineCount: len(NonEmptyLines(text)),
		}
	}
	text, previewLineCount := buildPreviewLines(anchors, fallback, maxLines)
	return readFileSummaryResult{
		Text:             text,
		RawLineCount:     rawLineCount,
		PreviewLineCount: previewLineCount,
	}
}

func buildPreviewLines(anchors, fallback []string, maxLines int) (string, int) {
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
	return JoinLimitedLines(out, maxLines), minInt(len(out), maxLines)
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

type listingSummaryResult struct {
	Text       string
	EntryCount int
	Grouped    bool
}

type treeOutputSummaryResult struct {
	Text    string
	Omitted bool
}

type readFileSummaryResult struct {
	Text             string
	RawLineCount     int
	PreviewLineCount int
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
