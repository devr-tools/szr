package cloudlist

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeAWSLambdaFunctions(input string, maxLines int) string {
	return summarizeAWSLambdaResult(input, maxLines).Text
}

func AWSLambdaFunctionsRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return cloudListRecovery(summarizeAWSLambdaResult(input, maxLines), "functions")
}

func summarizeAWSLambdaResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	records := awsCollection(input, "Functions")
	if len(records) == 0 {
		return summarizeCloudListResult(input, maxLines)
	}

	summaries := make([]string, len(records))
	runtimes := make([]string, len(records))
	for i, record := range records {
		summaries[i] = shared.Clip(summarizeLambdaFunction(record), 160)
		runtimes[i] = strings.ToLower(firstString(record, "Runtime"))
	}
	return awsListSummary("functions", summaries, runtimes, maxLines)
}

func summarizeLambdaFunction(record map[string]any) string {
	parts := []string{firstNonEmpty(firstString(record, "FunctionName"), lastSegment(firstString(record, "FunctionArn")))}
	if runtime := firstString(record, "Runtime"); runtime != "" {
		parts = append(parts, "runtime="+runtime)
	}
	if size, ok := firstNumber(record, "CodeSize"); ok {
		parts = append(parts, "size="+humanBytes(size))
	}
	if memory, ok := firstNumber(record, "MemorySize"); ok {
		parts = append(parts, fmt.Sprintf("mem=%.0fMB", memory))
	}
	if timeout, ok := firstNumber(record, "Timeout"); ok {
		parts = append(parts, fmt.Sprintf("timeout=%.0fs", timeout))
	}
	return strings.Join(parts, " ")
}

func SummarizeAWSECS(input string, maxLines int) string {
	return summarizeAWSECSResult(input, maxLines).Text
}

func AWSECSRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return cloudListRecovery(summarizeAWSECSResult(input, maxLines), "resources")
}

func summarizeAWSECSResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	decoded := awsObject(input)
	if decoded == nil {
		return summarizeCloudListResult(input, maxLines)
	}
	if services := objectSlice(decoded["services"]); len(services) > 0 {
		return summarizeECSServices(services, objectSlice(decoded["failures"]), maxLines)
	}
	for key, label := range map[string]string{"clusterArns": "clusters", "serviceArns": "services", "taskArns": "tasks"} {
		if arns := stringSlice(decoded[key]); len(arns) > 0 {
			return summarizeECSArns(label, arns, maxLines)
		}
	}
	return summarizeCloudListResult(input, maxLines)
}

func summarizeECSArns(label string, arns []string, maxLines int) cloudListSummaryResult {
	summaries := make([]string, len(arns))
	for i, arn := range arns {
		summaries[i] = lastSegment(arn)
	}
	return awsListSummary(label, summaries, make([]string, len(arns)), maxLines)
}

func summarizeECSServices(services, failures []map[string]any, maxLines int) cloudListSummaryResult {
	summaries := make([]string, 0, len(services)+len(failures))
	statuses := make([]string, 0, len(services)+len(failures))
	for _, failure := range failures {
		line := "failure: " + lastSegment(firstString(failure, "arn")) + " " + firstString(failure, "reason")
		summaries = append(summaries, shared.Clip(strings.TrimSpace(line), 160))
		statuses = append(statuses, "failure")
	}
	for _, service := range services {
		summaries = append(summaries, shared.Clip(summarizeECSService(service), 160))
		statuses = append(statuses, strings.ToLower(firstString(service, "status")))
	}
	return awsListSummary("services", summaries, statuses, maxLines)
}

func summarizeECSService(service map[string]any) string {
	parts := []string{firstNonEmpty(firstString(service, "serviceName"), lastSegment(firstString(service, "serviceArn")))}
	if status := firstString(service, "status"); status != "" {
		parts = append(parts, "status="+status)
	}
	running, _ := firstNumber(service, "runningCount")
	desired, _ := firstNumber(service, "desiredCount")
	parts = append(parts, fmt.Sprintf("running=%.0f/%.0f", running, desired))
	if pending, ok := firstNumber(service, "pendingCount"); ok && pending > 0 {
		parts = append(parts, fmt.Sprintf("pending=%.0f", pending))
	}
	if rollout := ecsRolloutState(service); rollout != "" && rollout != "COMPLETED" {
		parts = append(parts, "rollout="+rollout)
	}
	return strings.Join(parts, " ")
}

func ecsRolloutState(service map[string]any) string {
	deployments := objectSlice(service["deployments"])
	if len(deployments) == 0 {
		return ""
	}
	return firstString(deployments[0], "rolloutState")
}

// awsListSummary renders a header with a status breakdown plus per-record
// lines, keeping minority-status records ahead of positional truncation the
// same way the generic cloud inventory summary does.
func awsListSummary(label string, summaries, statuses []string, maxLines int) cloudListSummaryResult {
	header := fmt.Sprintf("%s: %d", label, len(summaries))
	if breakdown := statusBreakdown(statuses); breakdown != "" {
		header += " (" + breakdown + ")"
	}
	kept, omitted := selectRecordSummaries(summaries, statuses, maxLines)
	out := append([]string{header}, kept...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d more %s", omitted, label))
	}
	return cloudListSummaryResult{Text: strings.Join(out, "\n"), OmittedCount: omitted}
}

func cloudListRecovery(result cloudListSummaryResult, label string) (string, string, bool) {
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional %s", result.OmittedCount, label))
}

func awsObject(input string) map[string]any {
	trimmed := strings.TrimSpace(shared.StripANSI(input))
	if trimmed == "" || trimmed[0] != '{' {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	return decoded
}

func awsCollection(input, key string) []map[string]any {
	decoded := awsObject(input)
	if decoded == nil {
		return nil
	}
	return objectSlice(decoded[key])
}

func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
			out = append(out, strings.TrimSpace(text))
		}
	}
	return out
}

func firstNumber(record map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if value, ok := record[key].(float64); ok {
			return value, true
		}
		if text, ok := record[key].(string); ok {
			if value, err := strconv.ParseFloat(text, 64); err == nil {
				return value, true
			}
		}
	}
	return 0, false
}

func humanBytes(value float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	idx := 0
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%.0fB", value)
	}
	return fmt.Sprintf("%.1f%s", value, units[idx])
}

func sortBySizeDesc(entries []awsSizedLine) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].size > entries[j].size })
}

type awsSizedLine struct {
	size float64
	line string
}
