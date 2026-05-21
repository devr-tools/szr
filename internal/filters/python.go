package filters

import "strings"

func SummarizePytest(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := StripANSI(input)
	lines := nonEmptyLines(clean)
	if len(lines) == 0 {
		return "ok"
	}

	summaries := uniqueStrings(collectPytestSummaries(lines))
	failures := uniqueStrings(collectPytestShortFailures(lines))
	details := uniqueStrings(collectPytestDetails(lines))

	if len(failures) == 0 && len(details) == 0 {
		if len(summaries) > 0 {
			return joinLimitedLines(summaries, maxLines)
		}
		return CompactLines(clean, maxLines)
	}

	out := append([]string{}, summaries...)
	for _, line := range failures {
		out = append(out, clip(line, 160))
	}
	for _, line := range details {
		out = append(out, clip(line, 160))
	}
	return joinLimitedLines(out, maxLines)
}

func collectPytestSummaries(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		normalized := normalizePytestSummaryLine(trimmed)
		switch {
		case strings.HasPrefix(trimmed, "collected "):
			out = append(out, clip(trimmed, 160))
		case normalized == "no tests ran":
			out = append(out, normalized)
		case isPytestStatusSummary(normalized):
			out = append(out, clip(normalized, 160))
		}
	}
	return out
}

func collectPytestShortFailures(lines []string) []string {
	out := []string{}
	inSummary := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		normalized := normalizePytestSummaryLine(trimmed)

		if strings.HasPrefix(normalized, "FAILED ") || strings.HasPrefix(normalized, "ERROR ") {
			out = append(out, normalized)
			if !inSummary {
				continue
			}
		}

		if strings.EqualFold(normalized, "short test summary info") {
			inSummary = true
			continue
		}
		if !inSummary {
			continue
		}
		if isDividerLine(trimmed) {
			continue
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

func collectPytestDetails(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isInterestingPytestDetail(trimmed) {
			continue
		}
		out = append(out, normalizePytestDetail(trimmed))
	}
	return out
}

func normalizePytestSummaryLine(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.Trim(trimmed, "= -")
	return strings.TrimSpace(trimmed)
}

func isPytestStatusSummary(line string) bool {
	if line == "" {
		return false
	}
	statuses := []string{
		"failed",
		"passed",
		"error",
		"errors",
		"skipped",
		"xfailed",
		"xpassed",
		"warnings",
		"deselected",
	}
	hasStatus := false
	for _, status := range statuses {
		if strings.Contains(line, status) {
			hasStatus = true
			break
		}
	}
	if !hasStatus {
		return false
	}
	return strings.Contains(line, " in ") || strings.Contains(line, "seconds")
}

func isDividerLine(line string) bool {
	return strings.Trim(line, "=-_ ") == ""
}

func isInterestingPytestDetail(line string) bool {
	switch {
	case line == "":
		return false
	case strings.HasPrefix(line, "FAILED "),
		strings.HasPrefix(line, "ERROR "):
		return false
	case strings.HasPrefix(line, "E   "),
		strings.HasPrefix(line, "E       "),
		strings.HasPrefix(line, "assert "),
		strings.HasPrefix(line, ">") && strings.Contains(line, "assert"),
		strings.HasPrefix(line, ">") && strings.Contains(line, "available fixtures:"),
		strings.HasPrefix(line, "available fixtures:"):
		return true
	case strings.Contains(line, ".py:"),
		strings.Contains(line, "AssertionError"),
		strings.Contains(line, "ModuleNotFoundError"),
		strings.Contains(line, "ImportError"),
		strings.Contains(line, "TypeError"),
		strings.Contains(line, "NameError"),
		strings.Contains(line, "ValueError"),
		strings.Contains(line, "FileNotFoundError"),
		strings.Contains(line, "RuntimeError"),
		strings.Contains(line, "fixture '") && strings.Contains(line, "not found"):
		return true
	default:
		return false
	}
}

func normalizePytestDetail(line string) string {
	switch {
	case strings.HasPrefix(line, "E   "):
		return strings.TrimSpace(strings.TrimPrefix(line, "E"))
	case strings.HasPrefix(line, ">"):
		return strings.TrimSpace(strings.TrimPrefix(line, ">"))
	default:
		return line
	}
}
