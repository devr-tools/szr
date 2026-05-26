package cloudlogs

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

type event struct {
	Timestamp string
	Source    string
	Severity  string
	Message   string
}

func SummarizeCloudLogs(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	events := parseStructuredEvents(strings.TrimSpace(clean))
	if len(events) == 0 {
		events = parseTextEvents(clean)
	}
	if len(events) == 0 {
		return shared.CompactLines(clean, maxLines)
	}

	return renderEvents(events, maxLines)
}

func parseStructuredEvents(input string) []event {
	if input == "" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		return nil
	}

	var records []map[string]any
	switch typed := decoded.(type) {
	case []any:
		records = objectSlice(typed)
	case map[string]any:
		for _, key := range []string{"events", "entries", "value"} {
			if items := objectSlice(typed[key]); len(items) > 0 {
				records = items
				break
			}
		}
		if len(records) == 0 {
			records = []map[string]any{typed}
		}
	default:
		return nil
	}

	out := make([]event, 0, len(records))
	for _, record := range records {
		evt := event{
			Timestamp: firstString(record, "timestamp", "receiveTimestamp", "eventTimestamp", "timeGenerated"),
			Source: firstNonEmpty(
				firstString(record, "logName", "resourceGroupName"),
				firstString(nestedMap(record, "resource"), "type"),
				firstString(nestedMap(record, "resource"), "labels.container_name"),
				firstString(nestedMap(record, "resource"), "labels.instance_id"),
				nestedString(record, "resource", "labels", "container_name"),
				nestedString(record, "resource", "labels", "instance_id"),
				nestedString(record, "resource", "labels", "function_name"),
				nestedString(record, "resource", "labels", "project_id"),
				nestedString(record, "operationName", "localizedValue"),
			),
			Severity: firstNonEmpty(
				firstString(record, "severity", "level"),
				nestedString(record, "status", "localizedValue"),
			),
			Message: firstNonEmpty(
				firstString(record, "message", "textPayload"),
				nestedString(record, "jsonPayload", "message"),
				nestedString(record, "properties", "message"),
				nestedString(record, "properties", "statusMessage"),
				nestedString(record, "protoPayload", "status", "message"),
			),
		}
		if evt.Message == "" {
			evt.Message = firstInterestingValue(record)
		}
		if evt.Source == "" {
			evt.Source = "cloud"
		}
		out = append(out, evt)
	}
	return out
}

func parseTextEvents(input string) []event {
	lines := shared.NonEmptyLines(input)
	out := make([]event, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if evt, ok := parseTimestampedLine(trimmed); ok {
			out = append(out, evt)
			continue
		}
		if sharedLineInteresting(trimmed) {
			out = append(out, event{Source: "cloud", Message: trimmed})
		}
	}
	return out
}

func parseTimestampedLine(line string) (event, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return event{}, false
	}

	ts := ""
	restStart := 0
	if looksLikeTimestamp(fields[0]) {
		ts = fields[0]
		restStart = 1
		if len(fields) > 1 && looksLikeClock(fields[1]) {
			ts = ts + " " + fields[1]
			restStart = 2
		}
	}
	if ts == "" || restStart >= len(fields) {
		return event{}, false
	}

	source := "cloud"
	severity := ""
	messageStart := restStart
	if restStart < len(fields) {
		source = fields[restStart]
		messageStart++
	}
	if messageStart < len(fields) && isSeverityToken(fields[messageStart]) {
		severity = strings.ToUpper(strings.Trim(fields[messageStart], "[]:"))
		messageStart++
	}
	message := strings.Join(fields[messageStart:], " ")
	if message == "" {
		message = strings.Join(fields[restStart:], " ")
	}
	return event{
		Timestamp: ts,
		Source:    source,
		Severity:  severity,
		Message:   message,
	}, true
}

func renderEvents(events []event, maxLines int) string {
	firstTS := ""
	lastTS := ""
	sources := map[string]struct{}{}
	signatures := map[string]int{}
	order := []string{}

	for _, evt := range events {
		if evt.Timestamp != "" {
			if firstTS == "" || evt.Timestamp < firstTS {
				firstTS = evt.Timestamp
			}
			if lastTS == "" || evt.Timestamp > lastTS {
				lastTS = evt.Timestamp
			}
		}
		if evt.Source != "" {
			sources[evt.Source] = struct{}{}
		}
		if !sharedLineInteresting(strings.TrimSpace(evt.Severity + " " + evt.Message)) {
			continue
		}
		line := strings.TrimSpace(strings.Join([]string{evt.Source + ":", evt.Severity, evt.Message}, " "))
		line = strings.ReplaceAll(line, "  ", " ")
		if _, ok := signatures[line]; !ok {
			order = append(order, line)
		}
		signatures[line]++
	}

	out := []string{fmt.Sprintf("events: %d sources: %d", len(events), len(sources))}
	if firstTS != "" || lastTS != "" {
		out = append(out, fmt.Sprintf("time: %s .. %s", orDefault(firstTS, "unknown"), orDefault(lastTS, "unknown")))
	}
	if len(sources) > 0 {
		sourceList := make([]string, 0, len(sources))
		for source := range sources {
			sourceList = append(sourceList, source)
		}
		sort.Strings(sourceList)
		out = append(out, "services: "+strings.Join(sourceList, ", "))
	}

	if len(order) == 0 {
		for _, evt := range events {
			line := strings.TrimSpace(strings.Join([]string{evt.Source + ":", evt.Severity, evt.Message}, " "))
			line = strings.ReplaceAll(line, "  ", " ")
			if strings.TrimSpace(line) != ":" && strings.TrimSpace(line) != "" {
				order = append(order, line)
			}
			if len(order) >= maxLines-3 {
				break
			}
		}
	}

	for _, line := range order {
		count := signatures[line]
		if count > 1 {
			out = append(out, shared.Clip(fmt.Sprintf("%s (x%d)", line, count), 160))
			continue
		}
		out = append(out, shared.Clip(line, 160))
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func objectSlice(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, object)
	}
	return out
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if strings.Contains(key, ".") {
			continue
		}
		value, ok := record[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func nestedMap(record map[string]any, key string) map[string]any {
	value, _ := record[key].(map[string]any)
	return value
}

func nestedString(value any, path ...string) string {
	current := value
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = object[key]
	}
	text, _ := current.(string)
	return strings.TrimSpace(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstInterestingValue(record map[string]any) string {
	for _, key := range []string{"summary", "description", "statusMessage"} {
		if text := firstString(record, key); text != "" {
			return text
		}
	}
	return ""
}

func sharedLineInteresting(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "denied") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "throttl")
}

func looksLikeTimestamp(value string) bool {
	return strings.Contains(value, "T") || strings.Count(value, "-") == 2 || strings.Count(value, "/") == 2
}

func looksLikeClock(value string) bool {
	return strings.Count(value, ":") >= 2
}

func isSeverityToken(value string) bool {
	switch strings.ToUpper(strings.Trim(value, "[]:")) {
	case "DEBUG", "INFO", "NOTICE", "WARN", "WARNING", "ERROR", "ERR", "FATAL", "CRITICAL":
		return true
	default:
		return false
	}
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
