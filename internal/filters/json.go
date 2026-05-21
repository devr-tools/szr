package filters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func RenderJSONStructure(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid json"
	}
	return RenderValueStructure(value)
}

func RenderValueStructure(value any) string {
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
