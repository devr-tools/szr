package filters

import (
	"bytes"
	"fmt"
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

func ScannerDedupe(data []byte) string {
	return DedupeLines(string(data), 20)
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

func clip(input string, max int) string {
	return Clip(input, max)
}

func Clip(input string, max int) string {
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	return string(runes[:max]) + "..."
}

func uniqueStrings(values []string) []string {
	return UniqueStrings(values)
}

func UniqueStrings(values []string) []string {
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
