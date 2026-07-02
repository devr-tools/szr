package filters

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// maxTreeLines caps BuildTree output so huge directory walks stay bounded.
const maxTreeLines = 100

func BuildTree(paths []string, root string) string {
	type node struct {
		Name     string
		Children map[string]*node
	}

	rootNode := &node{Name: filepath.Base(root), Children: map[string]*node{}}
	for _, path := range paths {
		parts := splitTreeParts(root, path, filepath.Rel)
		current := rootNode
		for _, part := range parts {
			if shouldSkipTreePart(part) {
				continue
			}
			child, ok := current.Children[part]
			if !ok {
				child = &node{Name: part, Children: map[string]*node{}}
				current.Children[part] = child
			}
			current = child
		}
	}

	var render func(n *node, depth int) []string
	render = func(n *node, depth int) []string {
		label := n.Name
		// Collapse single-child directory chains (a/b/c/d/) into one line.
		if depth > 0 {
			collapsed := false
			for len(n.Children) == 1 {
				var only *node
				for _, child := range n.Children {
					only = child
				}
				if len(only.Children) == 0 {
					break
				}
				label += "/" + only.Name
				n = only
				collapsed = true
			}
			if collapsed {
				label += "/"
			}
		}
		indent := strings.Repeat("  ", depth)
		lines := []string{indent + label}
		keys := make([]string, 0, len(n.Children))
		for key := range n.Children {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lines = append(lines, render(n.Children[key], depth+1)...)
		}
		return lines
	}

	lines := render(rootNode, 0)
	if len(lines) > maxTreeLines {
		omitted := len(lines) - maxTreeLines
		lines = append(lines[:maxTreeLines], fmt.Sprintf("... +%d more", omitted))
	}
	return strings.Join(lines, "\n")
}

func splitTreeParts(root, path string, relFn func(string, string) (string, error)) []string {
	return SplitTreeParts(root, path, relFn)
}

func SplitTreeParts(root, path string, relFn func(string, string) (string, error)) []string {
	rel, err := relFn(root, path)
	if err != nil {
		return nil
	}
	return strings.FieldsFunc(rel, func(r rune) bool {
		return r == filepath.Separator || r == '/' || r == '\\'
	})
}

func shouldSkipTreePart(part string) bool {
	return ShouldSkipTreePart(part)
}

func ShouldSkipTreePart(part string) bool {
	return part == "." || part == ""
}
