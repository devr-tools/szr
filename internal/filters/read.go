package filters

import (
	"fmt"
	"strings"
)

func ReadLevel(data []byte, level string, lineNumbers bool, maxLines int) string {
	lines := strings.Split(string(data), "\n")
	filtered := make([]string, 0, len(lines))
	for i, raw := range lines {
		line := raw
		switch level {
		case "minimal":
			if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
		case "aggressive":
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
				line = collapseBlock(trimmed)
			}
		}
		if lineNumbers {
			line = fmt.Sprintf("%4d  %s", i+1, line)
		}
		filtered = append(filtered, line)
	}
	if maxLines > 0 && len(filtered) > maxLines {
		filtered = append(filtered[:maxLines], fmt.Sprintf("... +%d more lines", len(filtered)-maxLines))
	}
	return strings.Join(filtered, "\n")
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
