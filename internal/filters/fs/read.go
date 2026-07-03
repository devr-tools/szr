package fs

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/devr-tools/szr/internal/filters"
)

var goBlockMemberPattern = regexp.MustCompile(`^([A-Z][A-Za-z0-9_]*)\b`)

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

	if result, ok := summarizeLogFilePreview(path, data, maxLines); ok {
		return result
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
	if isSignatureModePath(path) {
		return summarizeCodeSignaturePreviewResult(string(data), maxLines)
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

type previewLine struct {
	Number int
	Text   string
}

func summarizeCodeSignaturePreviewResult(input string, maxLines int) readFileSummaryResult {
	lines := strings.Split(filters.StripANSI(input), "\n")
	anchors := []string{}
	rawLineCount := 0

	for idx := 0; idx < len(lines); idx++ {
		trimmed := strings.TrimSpace(lines[idx])
		if trimmed == "" {
			continue
		}
		rawLineCount++
		if next, handled := appendSignaturePreviewAnchors(lines, idx, trimmed, &anchors); handled {
			idx = next
		}
	}

	if len(anchors) == 0 {
		return fallbackCodeSignaturePreview(input, maxLines)
	}
	return signaturePreviewSummary(anchors, rawLineCount, maxLines)
}

//nolint:maintidx // This dispatcher intentionally keeps signature-preview classification in one place.
func appendSignaturePreviewAnchors(lines []string, idx int, trimmed string, anchors *[]string) (int, bool) {
	switch {
	case isShebangLine(trimmed) || isAnnotationLine(trimmed):
		*anchors = append(*anchors, formatPreviewLine(idx+1, trimmed))
		return idx, true
	case isCommentOnlyLine(trimmed):
		if isTodoLine(trimmed) {
			*anchors = append(*anchors, formatPreviewLine(idx+1, trimCommentMarkers(trimmed)))
		}
		return idx, true
	case isImportBlockStart(trimmed):
		return appendSignaturePreviewBlock(lines, idx, anchors, collectImportBlock)
	case isDeclarationBlockStart(trimmed):
		return appendSignaturePreviewBlock(lines, idx, anchors, collectDeclarationBlock)
	case isDeclarationStart(trimmed):
		entry, next := collectDeclarationSignature(lines, idx)
		*anchors = append(*anchors, formatPreviewLine(entry.Number, entry.Text))
		return next, true
	case isTodoLine(trimmed):
		*anchors = append(*anchors, formatPreviewLine(idx+1, trimCommentMarkers(trimmed)))
		return idx, true
	case isCodeAnchor(trimmed):
		*anchors = append(*anchors, formatPreviewLine(idx+1, filters.Clip(normalizePreservedLine(trimmed), 160)))
		return idx, true
	default:
		return idx, false
	}
}

type previewBlockCollector func([]string, int) ([]previewLine, int)

func appendSignaturePreviewBlock(lines []string, idx int, anchors *[]string, collect previewBlockCollector) (int, bool) {
	block, next := collect(lines, idx)
	appendPreviewBlock(anchors, block)
	return next, true
}

func appendPreviewBlock(anchors *[]string, block []previewLine) {
	for _, entry := range block {
		*anchors = append(*anchors, formatPreviewLine(entry.Number, entry.Text))
	}
}

func fallbackCodeSignaturePreview(input string, maxLines int) readFileSummaryResult {
	text := filters.ReadLevel([]byte(input), "aggressive", true, maxLines)
	return readFileSummaryResult{
		Text:             text,
		RawLineCount:     len(filters.NonEmptyLines(filters.StripANSI(input))),
		PreviewLineCount: len(filters.NonEmptyLines(text)),
	}
}

func signaturePreviewSummary(anchors []string, rawLineCount int, maxLines int) readFileSummaryResult {
	text, previewLineCount := buildPreviewLines(anchors, nil, maxLines)
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

func formatPreviewLine(number int, text string) string {
	return fmt.Sprintf("%4d  %s", number, filters.Clip(text, 160))
}

func normalizePreviewLine(line string) string {
	if strings.Contains(line, "{") && strings.Contains(line, "}") {
		return filters.CollapseBlock(line)
	}
	return line
}

func normalizePreservedLine(line string) string {
	line = normalizeWhitespace(stripTrailingComment(line))
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

func isCommentOnlyLine(line string) bool {
	return isCommentLine(line) && !isShebangLine(line) && !isAnnotationLine(line)
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

func isSignatureModePath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if name == "makefile" || name == "dockerfile" {
		return true
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".rs", ".java",
		".kt", ".kts", ".swift", ".php", ".c", ".cc", ".cpp", ".cxx", ".h",
		".hh", ".hpp", ".m", ".mm", ".cs", ".scala", ".sh", ".bash", ".zsh",
		".fish", ".lua", ".pl", ".ex", ".exs":
		return true
	default:
		return false
	}
}

func isShebangLine(line string) bool {
	return strings.HasPrefix(line, "#!")
}

func isAnnotationLine(line string) bool {
	return strings.HasPrefix(line, "@") ||
		strings.HasPrefix(line, "#[") ||
		strings.HasPrefix(line, "//go:") ||
		strings.HasPrefix(line, "// +build") ||
		strings.HasPrefix(line, "#pragma ") ||
		strings.HasPrefix(line, "#define ") ||
		strings.HasPrefix(line, "#include ") ||
		strings.HasPrefix(line, "#import ")
}

func isTodoLine(line string) bool {
	return strings.Contains(line, "TODO") || strings.Contains(line, "FIXME")
}

func collectImportBlock(lines []string, start int) ([]previewLine, int) {
	out := []previewLine{}
	trimmed := strings.TrimSpace(lines[start])
	out = append(out, previewLine{Number: start + 1, Text: normalizePreservedLine(trimmed)})

	if !startsDelimitedPreviewBlock(trimmed) {
		return out, start
	}

	depth := blockDelta(trimmed)
	for idx := start + 1; idx < len(lines); idx++ {
		line := strings.TrimSpace(lines[idx])
		if importBlockShouldStop(line, &depth) {
			return out, idx
		}
		appendImportBlockLine(&out, idx, line)
	}
	return out, len(lines) - 1
}

func collectDeclarationBlock(lines []string, start int) ([]previewLine, int) {
	header := normalizeWhitespace(strings.TrimSpace(stripTrailingComment(lines[start])))
	out := []previewLine{{Number: start + 1, Text: renderDeclarationBlockHeader(header)}}
	depth := blockDelta(strings.TrimSpace(lines[start]))

	for idx := start + 1; idx < len(lines); idx++ {
		line := strings.TrimSpace(lines[idx])
		if declarationBlockShouldStop(line, &depth) {
			return out, idx
		}
		appendDeclarationBlockLine(&out, idx, line)
	}
	return out, len(lines) - 1
}

func collectDeclarationSignature(lines []string, start int) (previewLine, int) {
	parts := []string{}
	last := start
	for idx := start; idx < len(lines); idx++ {
		raw := strings.TrimSpace(lines[idx])
		if skip, stop := declarationSignatureState(raw, idx, start); stop {
			break
		} else if skip {
			continue
		}
		rendered := normalizeSignaturePart(raw)
		if rendered == "" {
			continue
		}
		parts = append(parts, rendered)
		last = idx
		if declarationTerminated(rendered) || len(parts) >= 8 {
			break
		}
	}
	return previewLine{Number: start + 1, Text: renderDeclarationSignature(parts)}, last
}

func startsDelimitedPreviewBlock(trimmed string) bool {
	return strings.HasSuffix(trimmed, "(") || strings.HasSuffix(trimmed, "{")
}

func importBlockShouldStop(line string, depth *int) bool {
	*depth += blockDelta(line)
	return *depth <= 0
}

func appendImportBlockLine(out *[]previewLine, idx int, line string) {
	if line == "" || isCommentOnlyLine(line) {
		return
	}
	if rendered := strings.TrimSpace(stripTrailingComment(line)); rendered != "" {
		*out = append(*out, previewLine{Number: idx + 1, Text: normalizeWhitespace(rendered)})
	}
}

func declarationBlockShouldStop(line string, depth *int) bool {
	*depth += blockDelta(line)
	return *depth <= 0
}

func appendDeclarationBlockLine(out *[]previewLine, idx int, line string) {
	switch {
	case line == "":
		return
	case isTodoLine(line):
		*out = append(*out, previewLine{Number: idx + 1, Text: trimCommentMarkers(line)})
	default:
		if name, ok := goBlockMemberName(line); ok {
			*out = append(*out, previewLine{Number: idx + 1, Text: name})
		}
	}
}

func declarationSignatureState(raw string, idx int, start int) (skip bool, stop bool) {
	switch {
	case raw == "" && idx > start:
		return false, true
	case raw == "":
		return true, false
	case idx > start && isCommentOnlyLine(raw) && !isTodoLine(raw):
		return true, false
	default:
		return false, false
	}
}

func normalizeSignaturePart(raw string) string {
	return strings.TrimSpace(stripTrailingComment(raw))
}

func renderDeclarationSignature(parts []string) string {
	joined := normalizeWhitespace(strings.Join(parts, " "))
	switch {
	case strings.Contains(joined, "{") && strings.Contains(joined, "}"):
		return filters.CollapseBlock(joined)
	case strings.Contains(joined, "{"):
		return strings.TrimSpace(joined[:strings.Index(joined, "{")]) + " { ... }"
	case strings.Contains(joined, "=>"):
		return joined + " { ... }"
	default:
		return joined
	}
}

func declarationTerminated(line string) bool {
	if strings.Contains(line, "{") || strings.Contains(line, "=>") {
		return true
	}
	return strings.HasSuffix(line, ":") || strings.HasSuffix(line, ";")
}

func isImportBlockStart(line string) bool {
	for _, prefix := range []string{"import ", "from ", "use ", "require ", "#include ", "#import "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isDeclarationBlockStart(line string) bool {
	for _, prefix := range []string{"const (", "var (", "type ("} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isDeclarationStart(line string) bool {
	for _, prefix := range []string{
		"export async function ", "export function ", "export default function ",
		"export class ", "export interface ", "export type ", "export const ",
		"export let ", "export enum ", "package ", "module ", "namespace ",
		"type ", "interface ", "struct ", "enum ", "class ", "func ", "fn ",
		"def ", "const ", "var ", "let ", "async function ", "function ",
		"public ", "private ", "protected ", "static ", "final ", "trait ",
		"impl ", "protocol ", "extension ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return strings.Contains(line, "=>")
}

func stripTrailingComment(line string) string {
	for _, marker := range []string{" //", "\t//", " #", "\t#", " /*"} {
		if idx := strings.Index(line, marker); idx >= 0 {
			return strings.TrimSpace(line[:idx])
		}
	}
	return strings.TrimSpace(line)
}

func trimCommentMarkers(line string) string {
	line = strings.TrimSpace(line)
	for _, prefix := range []string{"//", "#", "/*", "*", "--"} {
		line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
	}
	line = strings.TrimSuffix(line, "*/")
	return strings.TrimSpace(line)
}

func normalizeWhitespace(line string) string {
	return strings.Join(strings.Fields(line), " ")
}

func blockDelta(line string) int {
	return strings.Count(line, "(") + strings.Count(line, "{") - strings.Count(line, ")") - strings.Count(line, "}")
}

func renderDeclarationBlockHeader(header string) string {
	switch {
	case strings.HasSuffix(header, "("):
		return header + " ... )"
	case strings.HasSuffix(header, "{"):
		return header + " ... }"
	default:
		return header
	}
}

func goBlockMemberName(line string) (string, bool) {
	match := goBlockMemberPattern.FindStringSubmatch(strings.TrimSpace(stripTrailingComment(line)))
	if len(match) != 2 {
		return "", false
	}
	name := match[1]
	if name == "" {
		return "", false
	}
	r := []rune(name)[0]
	if !unicode.IsUpper(r) {
		return "", false
	}
	return name, true
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
