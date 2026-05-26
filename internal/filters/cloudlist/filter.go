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

		for _, key := range []string{"Buckets", "Users", "Groups", "Projects", "projects", "functions", "Functions", "branches", "Branches", "apps", "Apps", "domains", "Domains", "Droplets", "droplets", "Instances", "instances", "data", "items", "Items", "value", "Value", "resources", "Resources"} {
			if records := objectSlice(typed[key]); len(records) > 0 {
				return normalizeCollectionLabel(key), records
			}
		}

		for _, key := range []string{"User", "Group", "Project", "Bucket", "Instance", "Droplet", "Resource", "server", "app"} {
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
	case "items", "Items", "value", "Value", "resources", "Resources", "data":
		return "resources"
	default:
		return strings.ToLower(key)
	}
}

func summarizeRecord(record map[string]any) string {
	if summary, ok := summarizeVercelRecord(record); ok {
		return summary
	}
	if summary, ok := summarizeSupabaseRecord(record); ok {
		return summary
	}
	if summary, ok := summarizeHerokuRecord(record); ok {
		return summary
	}

	kind := strings.ToLower(firstString(record, "kind", "type", "Type"))
	name := recordName(record)
	id := recordID(record)

	parts := []string{recordTitle(kind, name)}
	if idPart := recordIDPart(id, name); idPart != "" {
		parts = append(parts, idPart)
	}
	if location := recordLocation(record); location != "" {
		parts = append(parts, location)
	}
	if status := recordStatus(record); status != "" {
		parts = append(parts, "status="+status)
	}
	parts = appendRecordDetails(parts, record)
	return strings.Join(parts, " ")
}

func recordName(record map[string]any) string {
	return firstNonEmpty(
		firstString(record, "name", "Name", "displayName", "hostname"),
		tagValue(record, "Name"),
		firstString(record, "InstanceId"),
		lastSegment(firstString(record, "id", "Id", "selfLink", "ocid")),
	)
}

func recordID(record map[string]any) string {
	return firstNonEmpty(firstString(record, "id", "Id", "arn", "Arn", "ocid", "urn"), firstString(record, "InstanceId"))
}

func recordTitle(kind, name string) string {
	switch {
	case kind != "" && name != "":
		return kind + " " + name
	case name != "":
		return name
	case kind != "":
		return kind
	default:
		return "resource"
	}
}

func recordIDPart(id, name string) string {
	if id == "" || id == name || lastSegment(id) == name {
		return ""
	}
	return "id=" + id
}

func recordLocation(record map[string]any) string {
	if zone := firstNonEmpty(lastSegment(firstString(record, "zone", "Zone", "availability_domain")), firstString(nestedMap(record, "Placement"), "AvailabilityZone")); zone != "" {
		return "zone=" + zone
	}
	if region := firstString(record, "region", "Region", "region_slug"); region != "" {
		return "region=" + region
	}
	if location := firstString(record, "location", "Location"); location != "" {
		return "location=" + location
	}
	return ""
}

func recordStatus(record map[string]any) string {
	return firstNonEmpty(
		firstString(record, "status", "Status", "provisioningState", "lifecycleState", "healthStatus", "powerState"),
		firstString(nestedMap(record, "State"), "Name"),
		firstString(nestedMap(record, "properties"), "provisioningState"),
	)
}

func appendRecordDetails(parts []string, record map[string]any) []string {
	if project := firstString(record, "project", "projectId"); project != "" {
		parts = append(parts, "project="+project)
	}
	if framework := firstString(record, "framework"); framework != "" {
		parts = append(parts, "framework="+framework)
	}
	if group := firstString(record, "resourceGroup"); group != "" {
		parts = append(parts, "group="+group)
	}
	if created := firstString(record, "created_at", "time_created"); created != "" {
		parts = append(parts, "created="+created)
	}
	if ip := firstNonEmpty(firstString(record, "publicIp"), firstString(record, "PublicIpAddress"), firstString(record, "natIP")); ip != "" {
		parts = append(parts, "ip="+ip)
	}
	return parts
}

func summarizeVercelRecord(record map[string]any) (string, bool) {
	if firstString(record, "url", "readyState", "target") == "" && firstString(record, "framework") == "" {
		return "", false
	}

	name := firstNonEmpty(firstString(record, "name"), firstString(record, "url"), firstString(record, "id", "uid"))
	parts := []string{name}
	if id := firstNonEmpty(firstString(record, "uid"), firstString(record, "id")); id != "" && id != name {
		parts = append(parts, "id="+id)
	}
	if target := firstString(record, "target"); target != "" {
		parts = append(parts, "target="+target)
	}
	if state := firstNonEmpty(firstString(record, "readyState"), firstString(record, "state")); state != "" {
		parts = append(parts, "state="+state)
	}
	if url := firstString(record, "url"); url != "" && url != name {
		parts = append(parts, "url="+url)
	}
	if framework := firstString(record, "framework"); framework != "" {
		parts = append(parts, "framework="+framework)
	}
	if branch := nestedString(record, "meta", "githubCommitRef"); branch != "" {
		parts = append(parts, "branch="+branch)
	}
	if creator := nestedString(record, "creator", "username"); creator != "" {
		parts = append(parts, "creator="+creator)
	}
	if latest := firstStringFromSlice(record, "latestDeployments", "readyState"); latest != "" {
		parts = append(parts, "latest="+latest)
	}
	return strings.Join(parts, " "), true
}

func summarizeSupabaseRecord(record map[string]any) (string, bool) {
	kind := firstNonEmpty(firstString(record, "kind", "type"), inferSupabaseKind(record))
	if kind == "" {
		return "", false
	}
	name := firstNonEmpty(firstString(record, "name", "slug"), firstString(record, "project_ref", "ref"), firstString(record, "id"))
	parts := []string{kind + " " + name}
	if ref := firstNonEmpty(firstString(record, "project_ref", "ref"), firstString(record, "id")); ref != "" && ref != name {
		parts = append(parts, "project_ref="+ref)
	}
	if region := firstString(record, "region"); region != "" {
		parts = append(parts, "region="+region)
	}
	if status := firstString(record, "status"); status != "" {
		parts = append(parts, "status="+status)
	}
	if version := firstString(record, "version"); version != "" {
		parts = append(parts, "version="+version)
	}
	if verifyJWT := boolString(record["verify_jwt"]); verifyJWT != "" {
		parts = append(parts, "verify_jwt="+verifyJWT)
	}
	if linked := boolString(record["linked"]); linked != "" {
		parts = append(parts, "linked="+linked)
	}
	return strings.Join(parts, " "), true
}

func summarizeHerokuRecord(record map[string]any) (string, bool) {
	if nestedString(record, "region", "name") == "" && nestedString(record, "team", "name") == "" && firstString(record, "web_url") == "" && nestedString(record, "stack", "name") == "" {
		return "", false
	}
	name := firstNonEmpty(firstString(record, "name"), firstString(record, "id"))
	parts := []string{name}
	if id := firstString(record, "id"); id != "" && id != name {
		parts = append(parts, "id="+id)
	}
	if region := firstNonEmpty(nestedString(record, "region", "name"), firstString(record, "region")); region != "" {
		parts = append(parts, "region="+region)
	}
	if team := firstNonEmpty(nestedString(record, "team", "name"), nestedString(record, "organization", "name")); team != "" {
		parts = append(parts, "team="+team)
	}
	if space := nestedString(record, "space", "name"); space != "" {
		parts = append(parts, "space="+space)
	}
	if stack := nestedString(record, "stack", "name"); stack != "" {
		parts = append(parts, "stack="+stack)
	}
	if web := firstString(record, "web_url"); web != "" {
		parts = append(parts, "web="+web)
	}
	if maintenance := boolString(record["maintenance"]); maintenance != "" {
		parts = append(parts, "maintenance="+maintenance)
	}
	if internalRouting := boolString(record["internal_routing"]); internalRouting != "" {
		parts = append(parts, "internal_routing="+internalRouting)
	}
	return strings.Join(parts, " "), true
}

func nestedMap(record map[string]any, key string) map[string]any {
	value, _ := record[key].(map[string]any)
	return value
}

func nestedString(record map[string]any, path ...string) string {
	var current any = record
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

func firstStringFromSlice(record map[string]any, key, field string) string {
	items, ok := record[key].([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		return ""
	}
	return firstString(first, field)
}

func inferSupabaseKind(record map[string]any) string {
	switch {
	case firstString(record, "verify_jwt", "version", "entrypoint_path") != "":
		return "function"
	case firstString(record, "project_ref", "ref") != "":
		return "branch"
	case firstString(record, "region") != "" || firstString(record, "organization_id") != "":
		return "project"
	default:
		return ""
	}
}

func boolString(value any) string {
	boolean, ok := value.(bool)
	if !ok {
		return ""
	}
	if boolean {
		return "true"
	}
	return "false"
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
