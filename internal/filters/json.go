package filters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type prioritizedJSONKey struct {
	key      string
	priority int
}

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

func SummarizeJSONPreview(data []byte, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid json"
	}
	return SummarizeJSONValuePreview(value, maxLines)
}

func SummarizeJSONValuePreview(value any, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 8
	}

	lines := []string{}
	appendJSONPreview(&lines, "root", value, 0, maxLines)
	if len(lines) == 0 {
		return RenderValueStructure(value)
	}
	return JoinLimitedLines(lines, maxLines)
}

func appendJSONPreview(lines *[]string, path string, value any, depth int, maxLines int) {
	if len(*lines) >= maxLines {
		return
	}

	switch node := value.(type) {
	case map[string]any:
		keys := make([]prioritizedJSONKey, 0, len(node))
		for key := range node {
			keys = append(keys, prioritizedJSONKey{key: key, priority: jsonKeyPriority(key)})
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].priority != keys[j].priority {
				return keys[i].priority < keys[j].priority
			}
			return keys[i].key < keys[j].key
		})
		*lines = append(*lines, fmt.Sprintf("%s: object keys=%d", formatJSONPath(path), len(keys)))
		for _, entry := range keys {
			if len(*lines) >= maxLines {
				return
			}
			childPath := joinJSONPath(path, entry.key)
			appendJSONPreview(lines, childPath, node[entry.key], depth+1, maxLines)
		}
	case []any:
		*lines = append(*lines, summarizeJSONArrayLine(path, node))
		if len(node) == 0 || depth >= 1 || len(*lines) >= maxLines {
			return
		}
		switch first := node[0].(type) {
		case map[string]any, []any:
			appendJSONPreview(lines, path+"[0]", first, depth+1, maxLines)
		}
	default:
		*lines = append(*lines, fmt.Sprintf("%s=%s", formatJSONPath(path), previewScalar(node)))
	}
}

func summarizeJSONArrayLine(path string, items []any) string {
	parts := []string{fmt.Sprintf("%s: array len=%d", formatJSONPath(path), len(items))}
	if sample := summarizeJSONArraySample(items); sample != "" {
		parts = append(parts, "sample="+sample)
	}
	return strings.Join(parts, " ")
}

func summarizeJSONArraySample(items []any) string {
	if len(items) == 0 {
		return ""
	}
	limit := len(items)
	if limit > 3 {
		limit = 3
	}
	samples := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		samples = append(samples, previewArrayElement(items[i]))
	}
	if len(items) > limit {
		samples = append(samples, fmt.Sprintf("+%d more", len(items)-limit))
	}
	return strings.Join(samples, ", ")
}

func previewArrayElement(value any) string {
	switch node := value.(type) {
	case map[string]any:
		keys := make([]prioritizedJSONKey, 0, len(node))
		for key := range node {
			keys = append(keys, prioritizedJSONKey{key: key, priority: jsonKeyPriority(key)})
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].priority != keys[j].priority {
				return keys[i].priority < keys[j].priority
			}
			return keys[i].key < keys[j].key
		})
		if len(keys) > 3 {
			keys = keys[:3]
		}
		names := make([]string, 0, len(keys))
		for _, entry := range keys {
			names = append(names, entry.key)
		}
		return "object{" + strings.Join(names, ",") + "}"
	case []any:
		return fmt.Sprintf("array(len=%d)", len(node))
	default:
		return previewScalar(node)
	}
}

func previewScalar(value any) string {
	switch node := value.(type) {
	case string:
		return strconv.Quote(Clip(node, 48))
	case float64:
		return strconv.FormatFloat(node, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(node)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", node)
	}
}

func formatJSONPath(path string) string {
	if path == "root" {
		return "root"
	}
	return strings.TrimPrefix(path, "root.")
}

func joinJSONPath(base, key string) string {
	if base == "" {
		return key
	}
	if base == "root" {
		return "root." + key
	}
	return base + "." + key
}

func jsonKeyPriority(key string) int {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), " ", "_"))
	switch normalized {
	case "id", "ids", "uuid", "uid", "ocid", "arn", "name", "displayname", "display_name", "title", "slug":
		return 0
	case "status", "state", "phase", "health", "healthstatus", "health_status", "severity", "level":
		return 1
	case "timestamp", "time", "createdat", "created_at", "updatedat", "updated_at", "modifiedat", "modified_at":
		return 2
	case "error", "errors", "message", "reason", "code", "description":
		return 3
	default:
		switch {
		case strings.HasSuffix(normalized, "_id"), strings.HasSuffix(normalized, "id"):
			return 4
		case strings.Contains(normalized, "name"):
			return 5
		case strings.Contains(normalized, "status"), strings.Contains(normalized, "state"):
			return 6
		case strings.Contains(normalized, "time"), strings.Contains(normalized, "date"):
			return 7
		case strings.Contains(normalized, "error"), strings.Contains(normalized, "message"), strings.Contains(normalized, "reason"):
			return 8
		default:
			return 100
		}
	}
}
