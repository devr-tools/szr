package fs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

func SummarizeReadFile(path string, data []byte, maxLines int) string {
	return summarizeReadFileResult(path, data, maxLines).Text
}

func ReadFileRecoveryInfo(path string, data []byte, maxLines int) (string, string, bool) {
	result := summarizeReadFileResult(path, data, maxLines)
	if result.RawLineCount == 0 || result.PreviewLineCount >= result.RawLineCount {
		return filters.NoRecovery()
	}
	return filters.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.RawLineCount-result.PreviewLineCount))
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
			lines := filters.NonEmptyLines(filters.RenderJSONStructure(data))
			return readFileSummaryResult{
				Text:             filters.JoinLimitedLines(lines, limit),
				RawLineCount:     len(lines),
				PreviewLineCount: minInt(len(lines), limit),
			}
		}
	case ".md", ".markdown", ".mdx", ".txt", ".rst":
		return summarizeDocPreviewResult(string(data), maxLines)
	}
	return summarizeCodePreviewResult(string(data), maxLines)
}

func summarizeDocPreviewResult(input string, maxLines int) readFileSummaryResult {
	anchors := []string{}
	sectionLead := []string{}
	seenLead := map[string]struct{}{}
	headingSeen := false
	rawLineCount := 0
	for _, line := range filters.NonEmptyLines(filters.StripANSI(input)) {
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
			anchors = append(anchors, filters.Clip(trimmed, 160))
			headingSeen = strings.HasPrefix(trimmed, "#")
		default:
			if headingSeen {
				key := firstSentence(trimmed)
				if _, ok := seenLead[key]; !ok && key != "" {
					seenLead[key] = struct{}{}
					sectionLead = append(sectionLead, filters.Clip(key, 160))
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

func summarizeCodePreviewResult(input string, maxLines int) readFileSummaryResult {
	anchors := []string{}
	fallback := []string{}
	rawLineCount := 0
	for idx, raw := range strings.Split(filters.StripANSI(input), "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || isCommentLine(trimmed) {
			continue
		}
		rawLineCount++
		rendered := filters.Clip(normalizePreviewLine(trimmed), 160)
		line := fmt.Sprintf("%4d  %s", idx+1, rendered)
		if isCodeAnchor(trimmed) {
			anchors = append(anchors, line)
			continue
		}
		fallback = append(fallback, line)
	}

	if len(anchors) == 0 && len(fallback) == 0 {
		text := filters.ReadLevel([]byte(input), "minimal", true, maxLines)
		return readFileSummaryResult{
			Text:             text,
			RawLineCount:     len(filters.NonEmptyLines(filters.StripANSI(input))),
			PreviewLineCount: len(filters.NonEmptyLines(text)),
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

	appendUnique(filters.UniqueStrings(anchors))
	appendUnique(filters.UniqueStrings(fallback))
	return filters.JoinLimitedLines(out, maxLines), minInt(len(out), maxLines)
}

func normalizePreviewLine(line string) string {
	if strings.Contains(line, "{") && strings.Contains(line, "}") {
		return filters.CollapseBlock(line)
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

type readFileSummaryResult struct {
	Text             string
	RawLineCount     int
	PreviewLineCount int
}

func firstSentence(line string) string {
	line = strings.TrimSpace(line)
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(line, sep); idx >= 0 {
			return line[:idx+1]
		}
	}
	if len([]rune(line)) > 100 {
		return filters.Clip(line, 100)
	}
	return line
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
