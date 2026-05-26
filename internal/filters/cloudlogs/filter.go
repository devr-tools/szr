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
		for _, key := range []string{"events", "entries", "value", "results"} {
			if items := objectSlice(typed[key]); len(items) > 0 {
				records = items
				break
			}
		}
		if len(records) == 0 {
			if data, ok := typed["data"].(map[string]any); ok {
				for _, key := range []string{"events", "results"} {
					if items := objectSlice(data[key]); len(items) > 0 {
						records = items
						break
					}
				}
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
				firstString(record, "function_slug", "function_name", "entrypoint"),
				firstString(record, "source", "service", "name"),
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
				buildStructuredRequestMessage(record),
				firstString(record, "message", "textPayload"),
				firstString(record, "event_message", "error"),
				firstString(record, "msg", "details"),
				nestedString(record, "data", "message"),
				nestedString(record, "metadata", "message"),
				nestedString(record, "response", "message"),
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
		if evt, ok := parseHerokuLine(trimmed); ok {
			out = append(out, evt)
			continue
		}
		if evt, ok := parseVercelLine(trimmed); ok {
			out = append(out, evt)
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
		Source:    normalizeLogSource(source),
		Severity:  severity,
		Message:   message,
	}, true
}

func parseHerokuLine(line string) (event, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 || !looksLikeTimestamp(parts[0]) {
		return event{}, false
	}
	rest := strings.TrimSpace(parts[1])
	if !strings.Contains(rest, "[") || !strings.Contains(rest, "]:") {
		return event{}, false
	}
	source := "cloud"
	severity := ""
	message := rest
	if idx := strings.Index(rest, "]:"); idx >= 0 {
		prefix := rest[:idx+1]
		message = strings.TrimSpace(rest[idx+2:])
		source = normalizeHerokuSource(prefix)
	}
	if strings.HasPrefix(message, "Error ") {
		severity = "ERROR"
	}
	for _, token := range []string{" ERROR ", " FATAL ", " WARN "} {
		if strings.Contains(" "+message+" ", token) {
			severity = strings.TrimSpace(token)
			message = strings.Replace(message, token, " ", 1)
			message = strings.Join(strings.Fields(message), " ")
			break
		}
	}
	message = canonicalizeHerokuMessage(message)
	return event{Timestamp: parts[0], Source: source, Severity: severity, Message: message}, true
}

func parseVercelLine(line string) (event, bool) {
	fields := strings.Fields(line)
	if len(fields) < 3 || !looksLikeTimestamp(fields[0]) {
		return event{}, false
	}
	idx := 1
	source := ""
	if idx < len(fields) && !isSeverityToken(fields[idx]) {
		if idx+1 < len(fields) && !isSeverityToken(fields[idx+1]) {
			idx++
			source = fields[idx]
			idx++
		} else {
			source = fields[idx]
			idx++
		}
	}
	severity := ""
	if idx < len(fields) && isSeverityToken(fields[idx]) {
		severity = strings.ToUpper(strings.Trim(fields[idx], "[]:"))
		idx++
	}
	message := strings.Join(fields[idx:], " ")
	if message == "" {
		return event{}, false
	}
	return event{Timestamp: fields[0], Source: normalizeLogSource(source), Severity: severity, Message: canonicalizeVercelMessage(message)}, true
}

func renderEvents(events []event, maxLines int) string {
	stats := collectEventStats(events)
	out := []string{fmt.Sprintf("events: %d sources: %d", len(events), len(stats.sources))}
	if stats.firstTS != "" || stats.lastTS != "" {
		out = append(out, fmt.Sprintf("time: %s .. %s", orDefault(stats.firstTS, "unknown"), orDefault(stats.lastTS, "unknown")))
	}
	if len(stats.sources) > 0 {
		out = append(out, "services: "+strings.Join(sortedKeys(stats.sources), ", "))
	}

	if len(stats.order) == 0 {
		stats.order = fallbackEventLines(events, maxLines-3)
	}

	out = appendRenderedEvents(out, stats.order, stats.signatures)
	return shared.JoinLimitedLines(out, maxLines)
}

type eventStats struct {
	firstTS    string
	lastTS     string
	sources    map[string]struct{}
	signatures map[string]int
	order      []string
}

func collectEventStats(events []event) eventStats {
	stats := eventStats{
		sources:    map[string]struct{}{},
		signatures: map[string]int{},
	}
	for _, evt := range events {
		updateEventBounds(&stats, evt.Timestamp)
		if evt.Source != "" {
			stats.sources[evt.Source] = struct{}{}
		}
		if !sharedLineInteresting(strings.TrimSpace(evt.Severity + " " + evt.Message)) {
			continue
		}
		line := formatEventLine(evt)
		if _, ok := stats.signatures[line]; !ok {
			stats.order = append(stats.order, line)
		}
		stats.signatures[line]++
	}
	return stats
}

func updateEventBounds(stats *eventStats, timestamp string) {
	if timestamp == "" {
		return
	}
	if stats.firstTS == "" || timestamp < stats.firstTS {
		stats.firstTS = timestamp
	}
	if stats.lastTS == "" || timestamp > stats.lastTS {
		stats.lastTS = timestamp
	}
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fallbackEventLines(events []event, limit int) []string {
	if limit < 1 {
		limit = 1
	}
	capHint := limit
	if len(events) < capHint {
		capHint = len(events)
	}
	out := make([]string, 0, capHint)
	for _, evt := range events {
		line := formatEventLine(evt)
		if strings.TrimSpace(line) == ":" || strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func appendRenderedEvents(out, order []string, signatures map[string]int) []string {
	for _, line := range order {
		count := signatures[line]
		if count > 1 {
			out = append(out, shared.Clip(fmt.Sprintf("%s (x%d)", line, count), 160))
			continue
		}
		out = append(out, shared.Clip(line, 160))
	}
	return out
}

func formatEventLine(evt event) string {
	line := strings.TrimSpace(strings.Join([]string{evt.Source + ":", evt.Severity, evt.Message}, " "))
	return strings.ReplaceAll(line, "  ", " ")
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
	return containsInterestingFragment(lower)
}

var interestingLogFragments = []string{
	"error",
	"failed",
	"fatal",
	"panic",
	"exception",
	"denied",
	"timeout",
	"throttl",
	"h12",
	"h18",
	"r14",
	"r15",
	"l10",
	"l14",
	"unhandled",
	"uncaught",
	" 500",
	" 502",
	" 503",
	" 504",
	"oom",
	"out of memory",
}

func containsInterestingFragment(line string) bool {
	for _, fragment := range interestingLogFragments {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
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

func buildStructuredRequestMessage(record map[string]any) string {
	status := firstNonEmpty(firstString(record, "status_code", "statusCode"), nestedString(record, "response", "status_code"))
	method := firstString(record, "method")
	path := firstNonEmpty(firstString(record, "path"), nestedString(record, "metadata", "path"))
	msg := firstNonEmpty(firstString(record, "msg", "message", "event_message", "error"), nestedString(record, "metadata", "message"))
	parts := []string{}
	if status != "" {
		parts = append(parts, status)
	}
	if method != "" {
		parts = append(parts, method)
	}
	if path != "" {
		parts = append(parts, path)
	}
	if msg != "" {
		parts = append(parts, msg)
	}
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts, " ")
}

func normalizeHerokuSource(prefix string) string {
	prefix = strings.TrimSuffix(prefix, ":")
	switch {
	case strings.HasPrefix(prefix, "heroku[") && strings.HasSuffix(prefix, "]"):
		return "heroku/" + strings.TrimSuffix(strings.TrimPrefix(prefix, "heroku["), "]")
	case strings.HasPrefix(prefix, "app[") && strings.HasSuffix(prefix, "]"):
		return "app/" + strings.TrimSuffix(strings.TrimPrefix(prefix, "app["), "]")
	default:
		return normalizeLogSource(prefix)
	}
}

func normalizeLogSource(source string) string {
	source = strings.TrimSpace(strings.TrimSuffix(source, ":"))
	if source == "" {
		return "cloud"
	}
	return source
}

func canonicalizeHerokuMessage(message string) string {
	if strings.Contains(message, "code=H12") {
		return normalizeKeyValueMessage(message, []string{"code", "status", "dyno", "path", "method"})
	}
	if strings.Contains(message, "Error R14") {
		return "R14 Memory quota exceeded"
	}
	if strings.Contains(message, "Process exited with status") {
		return message
	}
	if strings.Contains(message, "State changed from") {
		return message
	}
	return message
}

func canonicalizeVercelMessage(message string) string {
	if strings.Contains(message, "GET /") || strings.Contains(message, "POST /") {
		return normalizeRequestLine(message)
	}
	return message
}

func normalizeRequestLine(message string) string {
	fields := strings.Fields(message)
	out := []string{}
	for _, field := range fields {
		if strings.HasPrefix(field, "req_") || strings.HasPrefix(strings.ToLower(field), "requestid=") {
			continue
		}
		out = append(out, field)
		if len(out) >= 4 {
			break
		}
	}
	return strings.Join(out, " ")
}

func normalizeKeyValueMessage(message string, keys []string) string {
	parts := []string{}
	for _, key := range keys {
		if value := extractKeyValue(message, key); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return message
	}
	return strings.Join(parts, " ")
}

func extractKeyValue(message, key string) string {
	marker := key + "="
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ""
	}
	value := message[idx+len(marker):]
	if strings.HasPrefix(value, "\"") {
		value = value[1:]
		if end := strings.Index(value, "\""); end >= 0 {
			return value[:end]
		}
		return value
	}
	if end := strings.IndexAny(value, " \t"); end >= 0 {
		return value[:end]
	}
	return value
}
