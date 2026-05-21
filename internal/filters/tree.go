package filters

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func BuildTree(paths []string, root string) string {
	type node struct {
		Name     string
		Children map[string]*node
		Files    int
	}

	rootNode := &node{Name: filepath.Base(root), Children: map[string]*node{}}
	for _, path := range paths {
		parts := splitTreeParts(root, path, filepath.Rel)
		current := rootNode
		for i, part := range parts {
			if shouldSkipTreePart(part) {
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
