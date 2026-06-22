package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const (
	ultraCompactSingleLineMaxTokens = 24
	ultraCompactSummaryMaxTokens    = 14
	ultraCompactDetailMaxTokens     = 24
)

func applyUltraCompactRender(inv Invocation, exec Execution, rendered string, rawCombined string) string {
	if !inv.UltraCompact {
		return rendered
	}
	lines, ok := normalizedUltraCompactLines(rendered)
	if !ok {
		return strings.TrimSpace(rendered)
	}
	return renderUltraCompactLines(lines, rawCombined, exec.ExitCode)
}

func compactNonEmptyLines(text string) []string {
	rawLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func ultraCompactDetailLines(renderedLines []string, rawCombined string, exitCode int) ([]string, int) {
	candidates := collectUltraCompactCandidates(renderedLines, rawCombined, exitCode)
	if len(candidates) == 0 {
		return nil, 0
	}
	selected, keptRendered := selectUltraCompactCandidates(candidates, ultraCompactMaxDetails(exitCode))
	if len(selected) == 0 {
		selected = append(selected, renderedLines[1])
		keptRendered = 1
	}
	return selected, keptRendered
}

func buildUltraCompactDetail(details []string, omitted int) string {
	parts := ultraCompactDetailParts(details, omitted)
	if len(parts) == 0 {
		return ""
	}
	return compressUltraCompactDetail(parts)
}

func normalizedUltraCompactLines(rendered string) ([]string, bool) {
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return nil, false
	}
	lines := compactNonEmptyLines(rendered)
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

func renderUltraCompactLines(lines []string, rawCombined string, exitCode int) string {
	if len(lines) == 1 {
		return hardCapTokens(lines[0], ultraCompactSingleLineMaxTokens)
	}
	summary := ultraCompactSummaryLine(lines[0])
	details, keptRendered := ultraCompactDetailLines(lines, rawCombined, exitCode)
	detail := buildUltraCompactDetail(details, len(lines)-1-keptRendered)
	if detail == "" {
		return summary
	}
	return summary + "\n" + detail
}

func ultraCompactSummaryLine(line string) string {
	summary := hardCapTokens(line, ultraCompactSummaryMaxTokens)
	if strings.TrimSpace(summary) == "" {
		return line
	}
	return summary
}

func ultraCompactDetailParts(details []string, omitted int) []string {
	parts := make([]string, 0, len(details)+1)
	for _, detail := range details {
		if trimmed := strings.TrimSpace(detail); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if omitted > 0 {
		parts = append(parts, "... +"+itoa(omitted)+" lines")
	}
	return parts
}

func compressUltraCompactDetail(parts []string) string {
	detail := strings.Join(parts, " | ")
	if history.EstimateTokens(detail) <= ultraCompactDetailMaxTokens {
		return detail
	}
	compressed := hardCapTokens(detail, ultraCompactDetailMaxTokens)
	if strings.TrimSpace(compressed) == "" {
		return parts[0]
	}
	return compressed
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return sign + string(buf[i:])
}
