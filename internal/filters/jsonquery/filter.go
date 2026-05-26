package jsonquery

import (
	"encoding/json"
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeQueryOutput(stdout, stderr string, maxLines int) string {
	return summarizeQueryOutputResult(stdout, stderr, maxLines).Text
}

func QueryOutputRecoveryInfo(stdout, stderr string, maxLines int) (string, string, bool) {
	result := summarizeQueryOutputResult(stdout, stderr, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type queryOutputSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeQueryOutputResult(stdout, stderr string, maxLines int) queryOutputSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	cleanStdout := strings.TrimSpace(shared.StripANSI(stdout))
	if rendered, ok, omitted := summarizeJSON(cleanStdout, maxLines); ok {
		return queryOutputSummaryResult{
			Text:         rendered,
			OmittedCount: omitted,
		}
	}

	cleanStderr := strings.TrimSpace(shared.StripANSI(stderr))
	if rendered, ok, omitted := summarizeJSON(cleanStderr, maxLines); ok {
		return queryOutputSummaryResult{
			Text:         rendered,
			OmittedCount: omitted,
		}
	}

	return queryOutputSummaryResult{
		Text: shared.CompactLines(strings.TrimSpace(joinStreams(cleanStdout, cleanStderr)), maxLines),
	}
}

func summarizeJSON(input string, maxLines int) (string, bool, int) {
	if input == "" {
		return "", false, 0
	}
	if rendered, ok := summarizeSingleJSON(input, maxLines); ok {
		full, fullOK := summarizeSingleJSON(input, largePreviewLimit(maxLines))
		if !fullOK {
			return rendered, true, 0
		}
		return rendered, true, omittedRenderedLines(full, rendered)
	}

	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false, 0
	}

	out := make([]string, 0, len(lines))
	fullOut := make([]string, 0, len(lines))
	perEntryLines := maxLines / len(lines)
	if perEntryLines < 3 {
		perEntryLines = 3
	}
	for _, line := range lines {
		rendered, ok := summarizeSingleJSON(strings.TrimSpace(line), perEntryLines)
		if !ok {
			return "", false, 0
		}
		rendered = trimRootSummary(rendered)
		out = append(out, rendered)
		fullRendered, fullOK := summarizeSingleJSON(strings.TrimSpace(line), largePreviewLimit(perEntryLines))
		if fullOK {
			fullOut = append(fullOut, trimRootSummary(fullRendered))
		}
	}
	rendered := shared.JoinLimitedLines(out, maxLines)
	fullRendered := strings.Join(fullOut, "\n")
	return rendered, true, omittedRenderedLines(fullRendered, rendered)
}

func summarizeSingleJSON(input string, maxLines int) (string, bool) {
	if !json.Valid([]byte(input)) {
		return "", false
	}
	return shared.SummarizeJSONPreview([]byte(input), maxLines), true
}

func joinStreams(stdout, stderr string) string {
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

func trimRootSummary(rendered string) string {
	lines := shared.NonEmptyLines(rendered)
	if len(lines) <= 1 {
		return rendered
	}
	if strings.HasPrefix(lines[0], "root: ") {
		return strings.Join(lines[1:], "\n")
	}
	return rendered
}

func largePreviewLimit(maxLines int) int {
	if maxLines < 12 {
		return 12
	}
	return maxLines * 8
}

func omittedRenderedLines(full, limited string) int {
	fullCount := len(shared.NonEmptyLines(full))
	limitedCount := len(shared.NonEmptyLines(limited))
	if fullCount <= limitedCount {
		return 0
	}
	return fullCount - limitedCount
}
