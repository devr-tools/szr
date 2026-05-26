package cloudlist

import (
	"encoding/json"
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeCloudList(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	trimmed := strings.TrimSpace(clean)
	if trimmed == "" {
		return "ok"
	}

	if summary, ok := summarizeStructured(trimmed, maxLines); ok {
		return summary
	}
	return shared.CompactLines(clean, maxLines)
}

func summarizeStructured(input string, maxLines int) (string, bool) {
	if input == "" {
		return "", false
	}
	switch input[0] {
	case '{', '[':
	default:
		return "", false
	}

	var decoded any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		return "", false
	}

	label, records := extractRecords(decoded)
	if len(records) == 0 {
		return "", false
	}

	out := []string{fmt.Sprintf("%s: %d", label, len(records))}
	for _, record := range records {
		out = append(out, shared.Clip(summarizeRecord(record), 160))
	}
	return shared.JoinLimitedLines(out, maxLines), true
}

func extractRecords(value any) (string, []map[string]any) {
	switch typed := value.(type) {
	case []any:
		return "resources", objectSlice(typed)
	case map[string]any:
		if reservations, ok := typed["Reservations"].([]any); ok {
			instances := []map[string]any{}
			for _, item := range reservations {
				reservation, ok := item.(map[string]any)
				if !ok {
					continue
				}
				instances = append(instances, objectSlice(reservation["Instances"])...)
			}
			if len(instances) > 0 {
				return "instances", instances
			}
		}

		for _, key := range []string{"Buckets", "Users", "Groups", "Projects", "items", "Items", "value", "Value", "resources", "Resources"} {
			if records := objectSlice(typed[key]); len(records) > 0 {
				return normalizeCollectionLabel(key), records
			}
		}

		for _, key := range []string{"User", "Group", "Project", "Bucket", "Instance", "Resource"} {
			if object, ok := typed[key].(map[string]any); ok {
				return "resource", []map[string]any{object}
			}
		}

		return "resource", []map[string]any{typed}
	default:
		return "", nil
	}
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

func normalizeCollectionLabel(key string) string {
	switch key {
	case "items", "Items", "value", "Value", "resources", "Resources":
		return "resources"
	default:
		return strings.ToLower(key)
	}
}

func summarizeRecord(record map[string]any) string {
	kind := strings.ToLower(firstString(record, "kind", "type", "Type"))
	name := firstNonEmpty(
		firstString(record, "name", "Name", "displayName"),
		tagValue(record, "Name"),
		firstString(record, "InstanceId"),
		lastSegment(firstString(record, "id", "Id", "selfLink")),
	)
	id := firstNonEmpty(firstString(record, "id", "Id", "arn", "Arn"), firstString(record, "InstanceId"))

	parts := []string{}
	switch {
	case kind != "" && name != "":
		parts = append(parts, kind+" "+name)
	case name != "":
		parts = append(parts, name)
	case kind != "":
		parts = append(parts, kind)
	default:
		parts = append(parts, "resource")
	}

	if id != "" && id != name && lastSegment(id) != name {
		parts = append(parts, "id="+id)
	}

	if zone := firstNonEmpty(lastSegment(firstString(record, "zone", "Zone")), firstString(nestedMap(record, "Placement"), "AvailabilityZone")); zone != "" {
		parts = append(parts, "zone="+zone)
	} else if region := firstString(record, "region", "Region"); region != "" {
		parts = append(parts, "region="+region)
	} else if location := firstString(record, "location", "Location"); location != "" {
		parts = append(parts, "location="+location)
	}

	if status := firstNonEmpty(
		firstString(record, "status", "Status", "provisioningState", "lifecycleState", "healthStatus", "powerState"),
		firstString(nestedMap(record, "State"), "Name"),
		firstString(nestedMap(record, "properties"), "provisioningState"),
	); status != "" {
		parts = append(parts, "status="+status)
	}

	if project := firstString(record, "project", "projectId"); project != "" {
		parts = append(parts, "project="+project)
	}
	if group := firstString(record, "resourceGroup"); group != "" {
		parts = append(parts, "group="+group)
	}
	if ip := firstNonEmpty(firstString(record, "publicIp"), firstString(record, "PublicIpAddress"), firstString(record, "natIP")); ip != "" {
		parts = append(parts, "ip="+ip)
	}

	return strings.Join(parts, " ")
}

func nestedMap(record map[string]any, key string) map[string]any {
	value, _ := record[key].(map[string]any)
	return value
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := record[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func tagValue(record map[string]any, key string) string {
	items, ok := record["Tags"].([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		tag, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if firstString(tag, "Key") == key {
			return firstString(tag, "Value")
		}
	}
	return ""
}

func lastSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, "/")
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx+1 < len(value) {
		return value[idx+1:]
	}
	return value
}
