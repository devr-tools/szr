package filters

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/filters/declarative"
)

func ReadLevel(data []byte, level string, lineNumbers bool, maxLines int) string {
	if level == "minimal" && !lineNumbers {
		return fastReadLevelMinimal(data, maxLines)
	}

	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines))
	for i, raw := range lines {
		line, keep := filterReadLine(raw, level)
		if !keep {
			continue
		}
		filtered = append(filtered, formatReadLine(line, i, lineNumbers))
	}
	filtered = limitReadLines(filtered, maxLines)
	return strings.Join(filtered, "\n")
}

func fastReadLevelMinimal(data []byte, maxLines int) string {
	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines))
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		filtered = append(filtered, raw)
	}
	filtered = limitReadLines(filtered, maxLines)
	return strings.Join(filtered, "\n")
}

func filterReadLine(raw string, level string) (string, bool) {
	switch level {
	case "minimal":
		return filterMinimalReadLine(raw)
	case "aggressive":
		return filterAggressiveReadLine(raw)
	default:
		return raw, true
	}
}

func filterMinimalReadLine(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	return raw, true
}

func filterAggressiveReadLine(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
		return collapseBlock(trimmed), true
	}
	return raw, true
}

func formatReadLine(line string, index int, lineNumbers bool) string {
	if !lineNumbers {
		return line
	}
	return fmt.Sprintf("%4d  %s", index+1, line)
}

func limitReadLines(lines []string, maxLines int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	return append(lines[:maxLines], fmt.Sprintf("... +%d more lines", len(lines)-maxLines))
}

func collapseBlock(line string) string {
	return CollapseBlock(line)
}

func CollapseBlock(line string) string {
	start := strings.Index(line, "{")
	end := strings.LastIndex(line, "}")
	if start < 0 || end <= start {
		return line
	}
	return strings.TrimSpace(line[:start]) + " { ... }"
}
