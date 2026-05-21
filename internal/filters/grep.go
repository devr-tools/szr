package filters

import (
	"bufio"
	"fmt"
	"strings"
)

func SummarizeRipgrep(input string, maxGroups, maxLines int) string {
	grouped := GroupRipgrep(input, maxGroups)
	if grouped != "no matches" {
		return grouped
	}
	return SummarizeGenericFailure(input, maxLines)
}

func GroupRipgrep(input string, maxGroups int) string {
	type match struct {
		Line int
		Text string
	}

	groups := map[string][]match{}
	order := []string{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		file := parts[0]
		text := strings.TrimSpace(parts[2])
		var lineNo int
		fmt.Sscanf(parts[1], "%d", &lineNo)
		if _, ok := groups[file]; !ok {
			order = append(order, file)
		}
		groups[file] = append(groups[file], match{Line: lineNo, Text: clip(text, 120)})
	}

	if len(order) == 0 {
		return "no matches"
	}

	var out []string
	for idx, file := range order {
		if idx >= maxGroups {
			out = append(out, fmt.Sprintf("... +%d more files", len(order)-maxGroups))
			break
		}
		out = append(out, fmt.Sprintf("%s (%d matches)", file, len(groups[file])))
		for i, m := range groups[file] {
			if i >= 3 {
				out = append(out, fmt.Sprintf("  ... +%d more", len(groups[file])-3))
				break
			}
			out = append(out, fmt.Sprintf("  %d: %s", m.Line, m.Text))
		}
	}
	return strings.Join(out, "\n")
}
