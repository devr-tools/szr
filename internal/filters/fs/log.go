package fs

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

// IsLogPreviewPath reports whether a file path is a candidate for the
// log-summary preview. Routing still requires log-shaped content, so plain
// prose .txt files keep the document preview.
func IsLogPreviewPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".log", ".out", ".txt":
		return true
	default:
		return false
	}
}

// summarizeLogFilePreview renders log-shaped file content as a severity
// histogram plus deduplicated messages. It reports ok=false when the path or
// content is not log-shaped so the caller can fall through to the regular
// document/code previews.
func summarizeLogFilePreview(path string, data []byte, maxLines int) (readFileSummaryResult, bool) {
	if !IsLogPreviewPath(path) {
		return readFileSummaryResult{}, false
	}
	input := string(data)
	if !filters.LooksLikeLogText(input) {
		return readFileSummaryResult{}, false
	}
	text, total := filters.SummarizeLogText(input, maxLines)
	return readFileSummaryResult{
		Text:             text,
		RawLineCount:     total,
		PreviewLineCount: len(filters.NonEmptyLines(text)),
	}, true
}
