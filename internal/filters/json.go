package filters

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	JSONModeStructure = "structure"
	JSONModePreview   = "preview"
)

type prioritizedJSONKey struct {
	key      string
	priority int
}

func RenderJSON(data []byte, mode string, maxLines int) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return "invalid json"
	}
	return RenderJSONValue(value, mode, maxLines)
}

func RenderJSONValue(value any, mode string, maxLines int) string {
	switch mode {
	case "", JSONModeStructure:
		return renderValueStructureLimited(value, maxLines)
	case JSONModePreview:
		return SummarizeJSONValuePreview(value, maxLines)
	default:
		return fmt.Sprintf("unsupported json mode: %s", mode)
	}
}

func RenderJSONStructure(data []byte) string {
	return RenderJSON(data, JSONModeStructure, 0)
}

const (
	// structureDefaultMaxLines bounds structure output when callers pass no
	// explicit budget; engine budgets clamp MaxLines to [3,40] so 40 is the
	// largest useful default.
	structureDefaultMaxLines = 40
	// structureMaxDepth bounds recursion; deeper nodes are summarized as
	// "{... N keys}" / "[N items]".
	structureMaxDepth = 4
)

func RenderValueStructure(value any) string {
	return renderValueStructureLimited(value, 0)
}

func renderValueStructureLimited(value any, maxLines int) string {
	if maxLines <= 0 {
		maxLines = structureDefaultMaxLines
	}
	r := &structureRenderer{maxLines: maxLines}
	r.render("", value, 0)
	if r.omitted > 0 {
		r.lines = append(r.lines, fmt.Sprintf("... +%d more lines", r.omitted))
	}
	return strings.Join(r.lines, "\n")
}

type structureRenderer struct {
	lines    []string
	maxLines int
	omitted  int
}

func (r *structureRenderer) full() bool {
	return len(r.lines) >= r.maxLines
}

func (r *structureRenderer) emit(line string) {
	if r.full() {
		r.omitted++
		return
	}
	r.lines = append(r.lines, line)
}

// render emits the structure of value, prefixing its first line with prefix
// (which carries the parent indentation and "key: " label, if any).
func (r *structureRenderer) render(prefix string, value any, depth int) {
	if r.full() {
		r.omitted += countStructureLines(value, depth)
		return
	}
	indent := strings.Repeat("  ", depth)
	switch node := value.(type) {
	case map[string]any:
		if len(node) == 0 {
			r.emit(prefix + "{}")
			return
		}
		if depth >= structureMaxDepth {
			r.emit(prefix + fmt.Sprintf("{... %d keys}", len(node)))
			return
		}
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
		r.emit(prefix + "{")
		for _, entry := range keys {
			r.render(indent+"  "+entry.key+": ", node[entry.key], depth+1)
		}
		r.emit(indent + "}")
	case []any:
		if len(node) == 0 {
			r.emit(prefix + "[]")
			return
		}
		if depth >= structureMaxDepth {
			r.emit(prefix + fmt.Sprintf("[%d items]", len(node)))
			return
		}
		r.emit(prefix + "[")
		r.render(indent+"  ", node[0], depth+1)
		if len(node) > 1 {
			r.emit(fmt.Sprintf("%s  ... +%d more items", indent, len(node)-1))
		}
		r.emit(indent + "]")
	case string:
		r.emit(prefix + "string")
	case float64:
		r.emit(prefix + "number")
	case bool:
		r.emit(prefix + "bool")
	case nil:
		r.emit(prefix + "null")
	default:
		r.emit(prefix + fmt.Sprintf("%T", node))
	}
}

// countStructureLines returns the number of lines render would emit for
// value at depth, without formatting them. Used to size the overflow marker
// once the line cap is reached.
func countStructureLines(value any, depth int) int {
	switch node := value.(type) {
	case map[string]any:
		if len(node) == 0 || depth >= structureMaxDepth {
			return 1
		}
		total := 2 // "{" and "}"
		for _, child := range node {
			total += countStructureLines(child, depth+1)
		}
		return total
	case []any:
		if len(node) == 0 || depth >= structureMaxDepth {
			return 1
		}
		total := 2 + countStructureLines(node[0], depth+1)
		if len(node) > 1 {
			total++ // "... +N more items"
		}
		return total
	default:
		return 1
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
		return renderValueStructureLimited(value, maxLines)
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
