package patch

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizePatchDiff(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	hunks := 0
	files := []string{}
	lines := []string{}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "@@"):
			hunks++
		case strings.HasPrefix(trimmed, "diff "),
			strings.HasPrefix(trimmed, "diff --git "):
			files = append(files, shared.Clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "--- "),
			strings.HasPrefix(trimmed, "+++ "):
			files = append(files, shared.Clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "Hunk #"),
			strings.HasPrefix(trimmed, "patching file "),
			strings.HasPrefix(trimmed, "error: patch failed:"),
			strings.Contains(trimmed, "FAILED"),
			strings.Contains(trimmed, "malformed patch"),
			strings.Contains(trimmed, "can't find file to patch"),
			strings.Contains(trimmed, ".rej"),
			strings.Contains(trimmed, "does not apply"),
			strings.Contains(trimmed, "patch unexpectedly ends"),
			strings.Contains(trimmed, "while searching for:"):
			lines = append(lines, shared.Clip(trimmed, 160))
		}
	}

	files = shared.UniqueStrings(files)
	lines = shared.UniqueStrings(lines)
	header := fmt.Sprintf("files=%d hunks=%d", len(files), hunks)
	if len(files) == 0 && hunks == 0 && len(lines) == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}

	out := []string{header}
	out = append(out, files...)
	out = append(out, lines...)
	return shared.JoinLimitedLines(out, maxLines)
}
