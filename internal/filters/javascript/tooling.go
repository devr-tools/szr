package javascript

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeJSTooling(input string, maxLines int) string {
	clean := StripANSI(input)
	lines := []string{}
	summaries := []string{}
	for _, line := range nonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "npm ERR!"),
			strings.HasPrefix(trimmed, "ERR_PNPM_"),
			strings.HasPrefix(trimmed, " ERR_PNPM_"),
			strings.HasPrefix(trimmed, "error Command"),
			strings.HasPrefix(trimmed, "vite v"),
			strings.HasPrefix(trimmed, "transforming"),
			strings.Contains(trimmed, "Tasks:"),
			strings.Contains(trimmed, "Failed Tasks:"),
			strings.Contains(trimmed, "Nx read the output"),
			strings.Contains(trimmed, "cache bypass"),
			strings.HasPrefix(trimmed, "turbo "),
			strings.HasPrefix(trimmed, "Failed projects:"),
			strings.HasPrefix(trimmed, "Failed tasks:"),
			strings.HasPrefix(trimmed, "warning"),
			strings.HasPrefix(trimmed, "Warnings:"):
			summaries = append(summaries, clip(trimmed, 160))
		case strings.Contains(trimmed, "error"),
			strings.Contains(trimmed, "Error:"),
			strings.Contains(trimmed, "failed"),
			strings.Contains(trimmed, ".ts:"),
			strings.Contains(trimmed, ".mts:"),
			strings.Contains(trimmed, ".cts:"),
			strings.Contains(trimmed, ".tsx:"),
			strings.Contains(trimmed, ".js:"),
			strings.Contains(trimmed, ".mjs:"),
			strings.Contains(trimmed, ".cjs:"),
			strings.Contains(trimmed, ".jsx:"),
			strings.Contains(trimmed, ".vue:"),
			strings.Contains(trimmed, ".svelte:"),
			strings.Contains(trimmed, "ELIFECYCLE"),
			strings.Contains(trimmed, "Cannot find module"),
			strings.Contains(trimmed, "RollupError"),
			strings.Contains(trimmed, "Type error:"),
			strings.Contains(trimmed, "Found ") && strings.Contains(trimmed, "error"),
			strings.Contains(trimmed, "esbuild"),
			strings.Contains(trimmed, "TS"):
			lines = append(lines, clip(trimmed, 160))
		}
	}

	lines = uniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return CompactLines(clean, maxLines)
	}

	out := append([]string{}, lines...)
	out = append(out, summaries...)
	return shared.JoinLimitedLines(out, maxLines)
}
