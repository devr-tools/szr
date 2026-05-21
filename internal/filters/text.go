package filters

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

func CompactLines(input string, maxLines int) string {
	lines := nonEmptyLines(input)
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	head := lines[:maxLines]
	return strings.Join(head, "\n") + fmt.Sprintf("\n... +%d more lines", len(lines)-maxLines)
}

func DedupeLines(input string, maxLines int) string {
	type item struct {
		Text  string
		Count int
	}
	order := []item{}
	index := map[string]int{}
	for _, line := range nonEmptyLines(input) {
		if pos, ok := index[line]; ok {
			order[pos].Count++
			continue
		}
		index[line] = len(order)
		order = append(order, item{Text: line, Count: 1})
	}

	var out []string
	for _, item := range order {
		line := item.Text
		if item.Count > 1 {
			line = fmt.Sprintf("%s (x%d)", line, item.Count)
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	if len(order) > maxLines {
		out = append(out, fmt.Sprintf("... +%d more unique lines", len(order)-maxLines))
	}
	return strings.Join(out, "\n")
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

func SummarizeGitStatus(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "clean"
	}

	branch := ""
	staged := 0
	unstaged := 0
	untracked := 0
	paths := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			branch = strings.TrimPrefix(line, "## ")
			continue
		}
		if len(line) < 3 {
			continue
		}
		x := line[0]
		y := line[1]
		path := strings.TrimSpace(line[3:])
		if path != "" && len(paths) < 6 {
			paths = append(paths, path)
		}
		switch {
		case x == '?' && y == '?':
			untracked++
		default:
			if x != ' ' {
				staged++
			}
			if y != ' ' {
				unstaged++
			}
		}
	}

	summary := []string{}
	if branch != "" {
		summary = append(summary, branch)
	}
	summary = append(summary, fmt.Sprintf("staged=%d unstaged=%d untracked=%d", staged, unstaged, untracked))
	if len(paths) > 0 {
		summary = append(summary, "files:")
		for _, path := range paths {
			summary = append(summary, "  "+path)
		}
	}
	return strings.Join(summary, "\n")
}

func SummarizeGitLog(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "no commits"
	}
	head := lines
	if len(head) > 10 {
		head = head[:10]
	}
	return fmt.Sprintf("%d commits\n%s", len(lines), strings.Join(head, "\n"))
}

func SummarizeGitDiff(input string) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "no diff"
	}
	fileCount := 0
	additions := 0
	deletions := 0
	summary := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			fileCount++
		}
		if strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			additions++
		} else if strings.HasPrefix(line, "-") {
			deletions++
		}
		if strings.Contains(line, "|") || strings.Contains(line, "files changed") {
			summary = append(summary, line)
		}
	}
	if len(summary) > 8 {
		summary = summary[:8]
	}
	header := fmt.Sprintf("files=%d +%d -%d", fileCount, additions, deletions)
	if len(summary) == 0 {
		return header + "\n" + CompactLines(input, 12)
	}
	return header + "\n" + strings.Join(summary, "\n")
}

func SummarizeGenericFailure(input string, maxLines int) string {
	lines := nonEmptyLines(input)
	if len(lines) == 0 {
		return "ok"
	}
	interesting := []string{}
	keywords := []string{"FAIL", "ERROR", "Error", "error", "panic", "warning", "Warning"}
	for _, line := range lines {
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				interesting = append(interesting, clip(line, 160))
				break
			}
		}
	}
	if len(interesting) == 0 {
		return CompactLines(input, maxLines)
	}
	if len(interesting) > maxLines {
		interesting = interesting[:maxLines]
	}
	return strings.Join(interesting, "\n")
}

func SummarizeGoTestJSON(input string) string {
	type event struct {
		Time    string `json:"Time"`
		Action  string `json:"Action"`
		Package string `json:"Package"`
		Test    string `json:"Test"`
		Output  string `json:"Output"`
	}

	type packageState struct {
		Passed bool
		Failed bool
	}

	failures := map[string][]string{}
	packages := map[string]*packageState{}
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		var ev event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Package != "" {
			if _, ok := packages[ev.Package]; !ok {
				packages[ev.Package] = &packageState{}
			}
		}
		switch ev.Action {
		case "fail":
			if ev.Test != "" {
				failures[ev.Package] = append(failures[ev.Package], ev.Test)
			} else if pkg := packages[ev.Package]; pkg != nil {
				pkg.Failed = true
			}
		case "pass":
			if ev.Test == "" {
				if pkg := packages[ev.Package]; pkg != nil {
					pkg.Passed = true
				}
			}
		case "output":
			if ev.Test != "" && strings.Contains(strings.ToLower(ev.Output), "panic") {
				failures[ev.Package] = append(failures[ev.Package], clip(strings.TrimSpace(ev.Output), 160))
			}
		}
	}

	if len(packages) == 0 {
		return CompactLines(input, 12)
	}

	passed := 0
	failed := 0
	for _, pkg := range packages {
		if pkg.Failed {
			failed++
		} else if pkg.Passed {
			passed++
		}
	}

	var out []string
	out = append(out, fmt.Sprintf("packages: pass=%d fail=%d", passed, failed))
	if len(failures) == 0 {
		out = append(out, "all tests passed")
		return strings.Join(out, "\n")
	}

	keys := make([]string, 0, len(failures))
	for key := range failures {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		unique := uniqueStrings(failures[key])
		out = append(out, fmt.Sprintf("%s", key))
		for i, testName := range unique {
			if i >= 4 {
				out = append(out, fmt.Sprintf("  ... +%d more", len(unique)-4))
				break
			}
			out = append(out, "  "+testName)
		}
	}
	return strings.Join(out, "\n")
}

func RenderJSONStructure(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid json"
	}
	return renderNode(value, 0)
}

func renderNode(value any, depth int) string {
	indent := strings.Repeat("  ", depth)
	switch node := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(node))
		for key := range node {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var lines []string
		lines = append(lines, indent+"{")
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s  %s: %s", indent, key, renderNode(node[key], depth+1)))
		}
		lines = append(lines, indent+"}")
		return strings.Join(lines, "\n")
	case []any:
		if len(node) == 0 {
			return "[]"
		}
		return "[\n" + renderNode(node[0], depth+1) + "\n" + indent + "]"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", node)
	}
}

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

func BuildTree(paths []string, root string) string {
	type node struct {
		Name     string
		Children map[string]*node
		Files    int
	}
	rootNode := &node{Name: filepath.Base(root), Children: map[string]*node{}}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		current := rootNode
		for i, part := range parts {
			if part == "." || part == "" {
				continue
			}
			child, ok := current.Children[part]
			if !ok {
				child = &node{Name: part, Children: map[string]*node{}}
				current.Children[part] = child
			}
			current = child
			if i == len(parts)-1 {
				current.Files++
			}
		}
	}
	var render func(n *node, depth int) []string
	render = func(n *node, depth int) []string {
		indent := strings.Repeat("  ", depth)
		lines := []string{indent + n.Name}
		keys := make([]string, 0, len(n.Children))
		for key := range n.Children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := n.Children[key]
			label := child.Name
			if child.Files > 0 && len(child.Children) == 0 {
				label = fmt.Sprintf("%s", label)
			}
			child.Name = label
			lines = append(lines, render(child, depth+1)...)
		}
		return lines
	}
	return strings.Join(render(rootNode, 0), "\n")
}

func ScannerDedupe(data []byte) string {
	return DedupeLines(string(data), 20)
}

func nonEmptyLines(input string) []string {
	raw := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}

func collapseBlock(line string) string {
	start := strings.Index(line, "{")
	end := strings.LastIndex(line, "}")
	if start < 0 || end <= start {
		return line
	}
	return strings.TrimSpace(line[:start]) + " { ... }"
}

func clip(input string, max int) string {
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "..."
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func StripANSI(input string) string {
	var out bytes.Buffer
	inEsc := false
	for _, r := range input {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && ((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')):
			inEsc = false
		case !inEsc:
			out.WriteRune(r)
		}
	}
	return out.String()
}
