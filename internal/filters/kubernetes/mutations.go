package kubernetes

import (
	"fmt"
	"path"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeApply(input string, maxLines int) string {
	return summarizeApplyResult(input, maxLines).Text
}

func SummarizeKubectlApply(input string, maxLines int) string {
	return SummarizeApply(input, maxLines)
}

func KubectlApplyRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeApplyResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

var applyResultVerbs = map[string]struct{}{
	"configured":         {},
	"unchanged":          {},
	"created":            {},
	"deleted":            {},
	"pruned":             {},
	"patched":            {},
	"replaced":           {},
	"serverside-applied": {},
}

func summarizeApplyResult(input string, maxLines int) kubernetesSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return kubernetesSummaryResult{Text: "ok"}
	}

	alerts := []string{}
	verbs := map[string][]string{}
	order := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isKubectlAlertLine(trimmed) {
			alerts = append(alerts, shared.Clip(trimmed, 160))
			continue
		}
		resource, verb, ok := parseApplyResultLine(trimmed)
		if !ok {
			continue
		}
		if _, seen := verbs[verb]; !seen {
			order = append(order, verb)
		}
		verbs[verb] = append(verbs[verb], resource)
	}

	alerts = shared.UniqueStrings(alerts)
	if len(order) == 0 {
		if len(alerts) == 0 {
			return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
		}
		return summarizeKubernetesLines(alerts, maxLines)
	}

	const sampleSize = 3
	headerParts := make([]string, 0, len(order))
	for _, verb := range order {
		headerParts = append(headerParts, fmt.Sprintf("%s=%d", verb, len(verbs[verb])))
	}
	out := []string{"resources: " + strings.Join(headerParts, " ")}
	out = append(out, alerts...)
	for _, verb := range order {
		names := verbs[verb]
		sample := names
		suffix := ""
		if len(names) > sampleSize {
			sample = names[:sampleSize]
			suffix = fmt.Sprintf(" +%d more", len(names)-sampleSize)
		}
		out = append(out, shared.Clip(fmt.Sprintf("%s: %s%s", verb, strings.Join(sample, ", "), suffix), 160))
	}
	return summarizeKubernetesLines(out, maxLines)
}

func parseApplyResultLine(line string) (string, string, bool) {
	trimmed := strings.TrimSuffix(line, " (dry run)")
	trimmed = strings.TrimSuffix(trimmed, " (server dry run)")
	fields := strings.Fields(trimmed)
	switch len(fields) {
	case 2:
		if _, ok := applyResultVerbs[fields[1]]; ok && strings.Contains(fields[0], "/") {
			return fields[0], fields[1], true
		}
	case 3:
		if _, ok := applyResultVerbs[fields[2]]; ok && strings.HasPrefix(fields[1], `"`) && strings.HasSuffix(fields[1], `"`) {
			return fields[0] + "/" + strings.Trim(fields[1], `"`), fields[2], true
		}
	}
	return "", "", false
}

func isKubectlAlertLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "error") ||
		strings.HasPrefix(lower, "warning") ||
		strings.Contains(line, "Error from server")
}

func SummarizeRollout(input string, maxLines int) string {
	return summarizeRolloutResult(input, maxLines).Text
}

func SummarizeKubectlRollout(input string, maxLines int) string {
	return SummarizeRollout(input, maxLines)
}

func KubectlRolloutRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeRolloutResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

func summarizeRolloutResult(input string, maxLines int) kubernetesSummaryResult {
	if maxLines <= 0 {
		maxLines = 8
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return kubernetesSummaryResult{Text: "ok"}
	}

	waitingCount := 0
	lastWaiting := ""
	finals := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Waiting for") && strings.Contains(trimmed, "rollout"):
			waitingCount++
			lastWaiting = trimmed
		case strings.Contains(trimmed, "successfully rolled out"),
			strings.Contains(trimmed, "exceeded its progress deadline"),
			isKubectlAlertLine(trimmed):
			finals = append(finals, shared.Clip(trimmed, 160))
		}
	}

	if waitingCount == 0 && len(finals) == 0 {
		return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}

	out := []string{}
	if waitingCount > 1 {
		out = append(out, fmt.Sprintf("progress: collapsed %d rollout updates", waitingCount-1))
	}
	if lastWaiting != "" {
		out = append(out, shared.Clip(lastWaiting, 160))
	}
	out = append(out, shared.UniqueStrings(finals)...)
	return summarizeKubernetesLines(out, maxLines)
}

func SummarizeDiff(input string, maxLines int) string {
	return summarizeDiffResult(input, maxLines).Text
}

func SummarizeKubectlDiff(input string, maxLines int) string {
	return SummarizeDiff(input, maxLines)
}

func KubectlDiffRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDiffResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

func summarizeDiffResult(input string, maxLines int) kubernetesSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	type diffObject struct {
		name    string
		added   int
		removed int
	}
	objects := []diffObject{}
	alerts := []string{}
	for _, line := range strings.Split(clean, "\n") {
		switch {
		case strings.HasPrefix(line, "diff "):
			fields := strings.Fields(line)
			name := ""
			if len(fields) > 0 {
				name = path.Base(fields[len(fields)-1])
			}
			objects = append(objects, diffObject{name: name})
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		case strings.HasPrefix(line, "+"):
			if len(objects) > 0 {
				objects[len(objects)-1].added++
			}
		case strings.HasPrefix(line, "-"):
			if len(objects) > 0 {
				objects[len(objects)-1].removed++
			}
		default:
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && isKubectlAlertLine(trimmed) {
				alerts = append(alerts, shared.Clip(trimmed, 160))
			}
		}
	}

	alerts = shared.UniqueStrings(alerts)
	if len(objects) == 0 {
		if len(alerts) > 0 {
			return summarizeKubernetesLines(alerts, maxLines)
		}
		return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}

	out := []string{fmt.Sprintf("diff: %d objects changed", len(objects))}
	out = append(out, alerts...)
	for _, object := range objects {
		out = append(out, shared.Clip(fmt.Sprintf("%s: +%d -%d", object.name, object.added, object.removed), 160))
	}
	return summarizeKubernetesLines(out, maxLines)
}
