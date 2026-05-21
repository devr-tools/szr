package build

import (
	"strings"

	shared "szr/internal/filters"
)

func SummarizeBuildSystem(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := []string{}
	summaries := []string{}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "make: ***"),
			strings.HasPrefix(trimmed, "just: error:"),
			strings.HasPrefix(trimmed, "task: Failed to run task"),
			strings.HasPrefix(trimmed, "FAILED:"),
			strings.HasPrefix(trimmed, "ERROR:"),
			strings.HasPrefix(trimmed, "ninja: error:"),
			strings.HasPrefix(trimmed, "ninja: build stopped:"),
			strings.HasPrefix(trimmed, "CMake Error"),
			strings.HasPrefix(trimmed, "Target //"),
			strings.HasPrefix(trimmed, "Build did NOT complete successfully"),
			strings.HasPrefix(trimmed, "FAILED: Build did NOT complete successfully"),
			strings.HasPrefix(trimmed, "FAILED: command succeeded but not all targets succeeded"):
			lines = append(lines, shared.Clip(trimmed, 160))
		case strings.Contains(trimmed, "No rule to make target"),
			strings.Contains(trimmed, "recipe for target"),
			strings.Contains(trimmed, "error generated"),
			strings.Contains(trimmed, "FAILED in"),
			strings.Contains(trimmed, "failed to solve"),
			strings.Contains(trimmed, "failed with exit code"),
			strings.Contains(trimmed, "failed:"),
			strings.Contains(trimmed, "error:"),
			strings.Contains(trimmed, ".cc:"),
			strings.Contains(trimmed, ".cpp:"),
			strings.Contains(trimmed, ".c:"),
			strings.Contains(trimmed, ".h:"),
			strings.Contains(trimmed, ".hpp:"),
			strings.Contains(trimmed, "undefined reference"):
			lines = append(lines, shared.Clip(trimmed, 160))
		case strings.HasPrefix(trimmed, "Built target "),
			strings.HasPrefix(trimmed, "["),
			strings.HasPrefix(trimmed, "Scanning dependencies"),
			strings.HasPrefix(trimmed, "-- Configuring"),
			strings.HasPrefix(trimmed, "-- Generating"),
			strings.HasPrefix(trimmed, "INFO: Analyzed"):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		}
	}

	lines = shared.UniqueStrings(shared.FoldConsecutiveLines(lines))
	summaries = shared.UniqueStrings(shared.FoldConsecutiveLines(summaries))
	if len(lines) == 0 && len(summaries) == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}

	anchors := []string{}
	other := []string{}
	for _, line := range lines {
		if shared.DiagnosticAnchor(line) != "" {
			anchors = append(anchors, line)
			continue
		}
		other = append(other, line)
	}

	out := append([]string{}, other...)
	out = append(out, shared.SelectUniqueAnchoredLines(anchors, maxLines/3+1)...)
	out = append(out, summaries...)
	return shared.JoinLimitedLines(out, maxLines)
}
