package installers

import "strings"

func mergeBlock(marker, body string) string {
	begin := "<!-- " + marker + ":begin -->"
	end := "<!-- " + marker + ":end -->"

	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return begin + "\n" + end + "\n"
	}
	return begin + "\n" + trimmed + "\n" + end + "\n"
}

func materialize(existing string, file File) string {
	if file.Strategy != StrategyMerge {
		return file.Content
	}

	begin := "<!-- " + file.Marker + ":begin -->"
	end := "<!-- " + file.Marker + ":end -->"
	block := mergeBlock(file.Marker, file.Content)

	if strings.Contains(existing, begin) && strings.Contains(existing, end) {
		start := strings.Index(existing, begin)
		finish := strings.Index(existing, end)
		if start >= 0 && finish >= start {
			finish += len(end)
			if finish < len(existing) && existing[finish] == '\n' {
				finish++
			}
			return existing[:start] + block + existing[finish:]
		}
	}

	trimmed := strings.TrimSpace(existing)
	if trimmed == "" {
		return block
	}
	return trimmed + "\n\n" + block
}
