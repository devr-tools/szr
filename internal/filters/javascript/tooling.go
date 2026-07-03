package javascript

import (
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeJSTooling(input string, maxLines int) string {
	clean := StripANSI(input)
	critical := []string{}
	lines := []string{}
	summaries := []string{}
	for _, line := range nonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case isJSRegistryErrorLine(trimmed):
			critical = append(critical, clip(decodeRegistrySpec(trimmed), 200))
		case strings.HasPrefix(trimmed, "npm ERR!"),
			strings.HasPrefix(trimmed, "ERR_PNPM_"),
			strings.HasPrefix(trimmed, " ERR_PNPM_"),
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

	critical = uniqueStrings(shared.FoldConsecutiveLines(critical))
	lines = uniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(critical) == 0 && len(lines) == 0 && len(summaries) == 0 {
		return CompactLines(clean, maxLines)
	}

	out := append([]string{}, critical...)
	out = append(out, lines...)
	out = append(out, summaries...)
	return shared.JoinLimitedLines(out, maxLines)
}

// isJSRegistryErrorLine recognizes package-manager registry failures whose
// package spec and error code must survive the render: pnpm fetch errors,
// npm 404/resolution failures, and yarn resolution errors.
func isJSRegistryErrorLine(line string) bool {
	if strings.HasPrefix(line, "ERR_PNPM_") {
		return true
	}
	if strings.HasPrefix(line, "npm ERR!") || strings.HasPrefix(line, "npm error") {
		for _, marker := range []string{"404", "E404", "ERESOLVE", "ENOTFOUND", "ETARGET", "registry"} {
			if strings.Contains(line, marker) {
				return true
			}
		}
		return false
	}
	for _, marker := range []string{
		"is not in the npm registry",
		"is not in this registry",
		"No authorization header was set",
		"Couldn't find package",
		"No matching version found for",
	} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

// decodeRegistrySpec rewrites URL-encoded scoped package specs
// ("@acme%2Fdesign-tokens") back to their plain form so the failing package
// name stays greppable in the render.
func decodeRegistrySpec(line string) string {
	if !strings.Contains(line, "%") {
		return line
	}
	replacer := strings.NewReplacer("%2F", "/", "%2f", "/", "%40", "@")
	return replacer.Replace(line)
}
