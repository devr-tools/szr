package cloudlist

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeCloudList(input string, maxLines int) string {
	return summarizeCloudListResult(input, maxLines).Text
}

func CloudListRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeCloudListResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional resources", result.OmittedCount))
}

type cloudListSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeCloudListResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	trimmed := strings.TrimSpace(clean)
	if trimmed == "" {
		return cloudListSummaryResult{Text: "ok"}
	}

	if summary, ok, omitted := summarizeStructured(trimmed, maxLines); ok {
		return cloudListSummaryResult{
			Text:         summary,
			OmittedCount: omitted,
		}
	}
	return cloudListSummaryResult{Text: shared.CompactLines(clean, maxLines)}
}

func summarizeStructured(input string, maxLines int) (string, bool, int) {
	if input == "" {
		return "", false, 0
	}
	switch input[0] {
	case '{', '[':
	default:
		return "", false, 0
	}

	var decoded any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		return "", false, 0
	}

	label, records := extractRecords(decoded)
	if len(records) == 0 {
		return "", false, 0
	}

	summaries := make([]string, len(records))
	statuses := make([]string, len(records))
	for i, record := range records {
		summaries[i] = shared.Clip(summarizeRecord(record), 160)
		statuses[i] = strings.ToLower(recordStatus(record))
	}

	header := fmt.Sprintf("%s: %d", label, len(records))
	if breakdown := statusBreakdown(statuses); breakdown != "" {
		header += " (" + breakdown + ")"
	}

	kept, omitted := selectRecordSummaries(summaries, statuses, maxLines)
	out := append([]string{header}, kept...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d more %s", omitted, label))
	}
	return strings.Join(out, "\n"), true, omitted
}

// statusBreakdown folds the per-record statuses into a "running=7 stopped=1"
// suffix (highest count first) so the state mix is visible even when not
// every record line fits the budget.
func statusBreakdown(statuses []string) string {
	counts, order := statusCounts(statuses)
	if len(order) < 2 {
		return ""
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	if len(order) > 4 {
		order = order[:4]
	}
	parts := make([]string, 0, len(order))
	for _, status := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
	}
	return strings.Join(parts, " ")
}

func statusCounts(statuses []string) (map[string]int, []string) {
	counts := map[string]int{}
	order := []string{}
	for _, status := range statuses {
		if status == "" {
			continue
		}
		if _, seen := counts[status]; !seen {
			order = append(order, status)
		}
		counts[status]++
	}
	return counts, order
}

// selectRecordSummaries keeps every record line when the budget allows, and
// otherwise minority-status records plus leading records. In inventory-style
// lists the anomalous entries (stopped among running, failed among succeeded)
// are the payload, so positional truncation must never be what drops them.
func selectRecordSummaries(summaries, statuses []string, maxLines int) ([]string, int) {
	limit := maxLines - 1
	if limit < 1 {
		limit = 1
	}
	if len(summaries) <= limit {
		return summaries, 0
	}
	keep := keepIndices(minorityStatusIndices(statuses), len(summaries), limit)
	out := filterByIndex(summaries, keep)
	return out, len(summaries) - len(out)
}

// keepIndices marks up to limit indices as kept: the anomalous ones first,
// then leading indices as positional fill.
func keepIndices(anomalies []int, total, limit int) map[int]bool {
	keep := map[int]bool{}
	for _, idx := range anomalies {
		if len(keep) >= limit {
			break
		}
		keep[idx] = true
	}
	for i := 0; i < total && len(keep) < limit; i++ {
		keep[i] = true
	}
	return keep
}

func filterByIndex(items []string, keep map[int]bool) []string {
	out := make([]string, 0, len(keep))
	for i, item := range items {
		if keep[i] {
			out = append(out, item)
		}
	}
	return out
}

// minorityStatusIndices reports the records whose status differs from the
// dominant status, provided a dominant status exists (covers more than half
// of the status-bearing records).
func minorityStatusIndices(statuses []string) []int {
	counts, _ := statusCounts(statuses)
	dominant, dominantCount, total := "", 0, 0
	for status, count := range counts {
		total += count
		if count > dominantCount {
			dominant, dominantCount = status, count
		}
	}
	if dominant == "" || dominantCount*2 <= total {
		return nil
	}
	out := []int{}
	for i, status := range statuses {
		if status != "" && status != dominant {
			out = append(out, i)
		}
	}
	return out
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
