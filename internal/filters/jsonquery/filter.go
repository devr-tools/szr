package jsonquery

import (
	"encoding/json"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeQueryOutput(stdout, stderr string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	cleanStdout := strings.TrimSpace(shared.StripANSI(stdout))
	if rendered, ok := summarizeJSON(cleanStdout, maxLines); ok {
		return rendered
	}

	cleanStderr := strings.TrimSpace(shared.StripANSI(stderr))
	if rendered, ok := summarizeJSON(cleanStderr, maxLines); ok {
		return rendered
	}

	return shared.CompactLines(strings.TrimSpace(joinStreams(cleanStdout, cleanStderr)), maxLines)
}

func summarizeJSON(input string, maxLines int) (string, bool) {
	if input == "" {
		return "", false
	}
	if rendered, ok := summarizeSingleJSON(input, maxLines); ok {
		return rendered, true
	}

	lines := shared.NonEmptyLines(input)
	if len(lines) == 0 {
		return "", false
	}

	out := make([]string, 0, len(lines))
	perEntryLines := maxLines / len(lines)
	if perEntryLines < 3 {
		perEntryLines = 3
	}
	for _, line := range lines {
		rendered, ok := summarizeSingleJSON(strings.TrimSpace(line), perEntryLines)
		if !ok {
			return "", false
		}
		rendered = trimRootSummary(rendered)
		out = append(out, rendered)
	}
	return shared.JoinLimitedLines(out, maxLines), true
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
