package jsonquery

import (
	"bytes"
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
	for _, line := range lines {
		rendered, ok := summarizeSingleJSON(strings.TrimSpace(line), 1)
		if !ok {
			return "", false
		}
		out = append(out, rendered)
	}
	return shared.JoinLimitedLines(out, maxLines), true
}

func summarizeSingleJSON(input string, maxLines int) (string, bool) {
	if !json.Valid([]byte(input)) {
		return "", false
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(input)); err != nil {
		return "", false
	}
	if maxLines <= 1 {
		return compact.String(), true
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "  "); err != nil {
		return compact.String(), true
	}

	lines := shared.NonEmptyLines(pretty.String())
	if len(lines) == 0 {
		return compact.String(), true
	}
	return shared.JoinLimitedLines(lines, maxLines), true
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
