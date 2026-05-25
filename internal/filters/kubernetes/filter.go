package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeGet(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	if summary, ok := summarizeGetJSON(clean, maxLines); ok {
		return summary
	}
	return shared.CompactLines(clean, maxLines)
}

func SummarizeKubectlGet(input string, maxLines int) string {
	return SummarizeGet(input, maxLines)
}

func SummarizeDescribe(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return "ok"
	}

	meta := []string{}
	events := []string{}
	inEvents := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Events:"):
			inEvents = true
		case isDescribeMeta(trimmed):
			meta = append(meta, shared.Clip(trimmed, 160))
		case inEvents && isInterestingEvent(trimmed):
			events = append(events, shared.Clip(trimmed, 160))
		}
	}

	meta = shared.UniqueStrings(meta)
	events = shared.UniqueStrings(events)
	if len(events) == 0 {
		return shared.JoinLimitedLines(meta, maxLines)
	}
	return shared.JoinLimitedLines(append(meta, events...), maxLines)
}

func SummarizeKubectlDescribe(input string, maxLines int) string {
	return SummarizeDescribe(input, maxLines)
}

func SummarizeLogs(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return "ok"
	}

	sources := map[string]map[string]int{}
	order := []string{}
	fallback := []string{}
	for _, line := range lines {
		source, message := splitLogLine(line)
		if !interestingLogLine(message) {
			if len(fallback) < maxLines {
				fallback = append(fallback, shared.Clip(strings.TrimSpace(line), 160))
			}
			continue
		}
		if _, ok := sources[source]; !ok {
			sources[source] = map[string]int{}
			order = append(order, source)
		}
		sources[source][shared.Clip(strings.TrimSpace(message), 160)]++
	}

	if len(order) == 0 {
		return shared.JoinLimitedLines(shared.UniqueStrings(fallback), maxLines)
	}

	out := []string{fmt.Sprintf("sources: %d", len(order))}
	for _, source := range order {
		for _, message := range sortedMessages(sources[source]) {
			line := fmt.Sprintf("%s: %s", source, message)
			if sources[source][message] > 1 {
				line = fmt.Sprintf("%s (x%d)", line, sources[source][message])
			}
			out = append(out, line)
		}
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func SummarizeKubectlLogs(input string, maxLines int) string {
	return SummarizeLogs(input, maxLines)
}

func summarizeGetJSON(input string, maxLines int) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed[0] != '{' {
		return "", false
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return "", false
	}

	kind, _ := decoded["kind"].(string)
	items, hasItems := decoded["items"].([]any)
	if hasItems {
		head := fmt.Sprintf("%s: %d items", strings.ToLower(strings.TrimSuffix(kind, "List")), len(items))
		out := []string{head}
		for _, item := range items {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, shared.Clip(summarizeObject(obj), 160))
		}
		return shared.JoinLimitedLines(out, maxLines), true
	}

	return shared.JoinLimitedLines([]string{shared.Clip(summarizeObject(decoded), 160)}, maxLines), true
}

func summarizeObject(obj map[string]any) string {
	kind, _ := obj["kind"].(string)
	meta, _ := obj["metadata"].(map[string]any)
	name := stringField(meta, "name")
	namespace := stringField(meta, "namespace")
	status, _ := obj["status"].(map[string]any)
	spec, _ := obj["spec"].(map[string]any)

	parts := []string{strings.ToLower(kind), name}
	if namespace != "" {
		parts = append(parts, "ns="+namespace)
	}
	if phase := stringField(status, "phase"); phase != "" {
		parts = append(parts, "phase="+phase)
	}
	if ready := podReadySummary(status); ready != "" {
		parts = append(parts, "ready="+ready)
	}
	if restarts := podRestartSummary(status); restarts != "" {
		parts = append(parts, "restarts="+restarts)
	}
	if replicas := replicaSummary(status); replicas != "" {
		parts = append(parts, replicas)
	}
	if serviceType := stringField(spec, "type"); serviceType != "" {
		parts = append(parts, "type="+serviceType)
	}
	if clusterIP := stringField(spec, "clusterIP"); clusterIP != "" {
		parts = append(parts, "clusterIP="+clusterIP)
	}

	return strings.Join(parts, " ")
}

func podReadySummary(status map[string]any) string {
	statuses, ok := status["containerStatuses"].([]any)
	if !ok || len(statuses) == 0 {
		return ""
	}
	ready := 0
	for _, item := range statuses {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if value, ok := entry["ready"].(bool); ok && value {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, len(statuses))
}

func podRestartSummary(status map[string]any) string {
	statuses, ok := status["containerStatuses"].([]any)
	if !ok || len(statuses) == 0 {
		return ""
	}
	total := 0
	for _, item := range statuses {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		total += int(numberField(entry, "restartCount"))
	}
	return fmt.Sprintf("%d", total)
}

func replicaSummary(status map[string]any) string {
	ready := int(numberField(status, "readyReplicas"))
	desired := int(numberField(status, "replicas"))
	available := int(numberField(status, "availableReplicas"))
	if desired == 0 && ready == 0 && available == 0 {
		return ""
	}
	return fmt.Sprintf("replicas=%d/%d available=%d", ready, desired, available)
}

func isDescribeMeta(line string) bool {
	prefixes := []string{"Name:", "Namespace:", "Priority:", "Node:", "Labels:", "Status:", "IP:", "Controlled By:", "Image:", "Reason:"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isInterestingEvent(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "warning") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "back-off") ||
		strings.Contains(lower, "errimagepull") ||
		strings.Contains(lower, "crashloopbackoff") ||
		strings.Contains(lower, "unhealthy")
}

func splitLogLine(line string) (string, string) {
	fields := strings.Fields(line)
	if len(fields) >= 2 && strings.Contains(fields[0], "/") {
		return fields[0], strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
	}
	return "log", strings.TrimSpace(line)
}

func sortedMessages(counts map[string]int) []string {
	order := make([]string, 0, len(counts))
	for message := range counts {
		order = append(order, message)
	}
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if order[j] < order[i] {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	return order
}

func interestingLogLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "warn") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "refused") ||
		strings.Contains(lower, "unhealthy") ||
		strings.Contains(lower, "traceback")
}

func stringField(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	value, _ := obj[key].(string)
	return value
}

func numberField(obj map[string]any, key string) float64 {
	if obj == nil {
		return 0
	}
	switch value := obj[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	default:
		return 0
	}
}
