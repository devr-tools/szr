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

type applyScan struct {
	alerts []string
	verbs  map[string][]string
	order  []string
}

func scanApplyLines(lines []string) applyScan {
	scan := applyScan{verbs: map[string][]string{}}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isKubectlAlertLine(trimmed) {
			scan.alerts = append(scan.alerts, shared.Clip(trimmed, 160))
			continue
		}
		resource, verb, ok := parseApplyResultLine(trimmed)
		if !ok {
			continue
		}
		if _, seen := scan.verbs[verb]; !seen {
			scan.order = append(scan.order, verb)
		}
		scan.verbs[verb] = append(scan.verbs[verb], resource)
	}
	scan.alerts = shared.UniqueStrings(scan.alerts)
	return scan
}

func renderApplyHeaderLine(scan applyScan) string {
	headerParts := make([]string, 0, len(scan.order))
	for _, verb := range scan.order {
		headerParts = append(headerParts, fmt.Sprintf("%s=%d", verb, len(scan.verbs[verb])))
	}
	return "resources: " + strings.Join(headerParts, " ")
}

func renderApplyVerbLine(verb string, names []string) string {
	const sampleSize = 3
	sample := names
	suffix := ""
	if len(names) > sampleSize {
		sample = names[:sampleSize]
		suffix = fmt.Sprintf(" +%d more", len(names)-sampleSize)
	}
	return shared.Clip(fmt.Sprintf("%s: %s%s", verb, strings.Join(sample, ", "), suffix), 160)
}

func renderApplyScan(scan applyScan, maxLines int) kubernetesSummaryResult {
	out := []string{renderApplyHeaderLine(scan)}
	out = append(out, scan.alerts...)
	for _, verb := range scan.order {
		out = append(out, renderApplyVerbLine(verb, scan.verbs[verb]))
	}
	return summarizeKubernetesLines(out, maxLines)
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

	scan := scanApplyLines(lines)
	if len(scan.order) == 0 {
		if len(scan.alerts) == 0 {
			return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
		}
		return summarizeKubernetesLines(scan.alerts, maxLines)
	}
	return renderApplyScan(scan, maxLines)
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

type rolloutScan struct {
	waitingCount int
	lastWaiting  string
	finals       []string
}

func scanRolloutLines(lines []string) rolloutScan {
	scan := rolloutScan{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case isRolloutWaitingLine(trimmed):
			scan.waitingCount++
			scan.lastWaiting = trimmed
		case isRolloutFinalLine(trimmed):
			scan.finals = append(scan.finals, shared.Clip(trimmed, 160))
		}
	}
	return scan
}

func isRolloutWaitingLine(line string) bool {
	return strings.HasPrefix(line, "Waiting for") && strings.Contains(line, "rollout")
}

func isRolloutFinalLine(line string) bool {
	return strings.Contains(line, "successfully rolled out") ||
		strings.Contains(line, "exceeded its progress deadline") ||
		isKubectlAlertLine(line)
}

func renderRolloutScan(scan rolloutScan, maxLines int) kubernetesSummaryResult {
	out := []string{}
	if scan.waitingCount > 1 {
		out = append(out, fmt.Sprintf("progress: collapsed %d rollout updates", scan.waitingCount-1))
	}
	if scan.lastWaiting != "" {
		out = append(out, shared.Clip(scan.lastWaiting, 160))
	}
	out = append(out, shared.UniqueStrings(scan.finals)...)
	return summarizeKubernetesLines(out, maxLines)
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

	scan := scanRolloutLines(lines)
	if scan.waitingCount == 0 && len(scan.finals) == 0 {
		return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	return renderRolloutScan(scan, maxLines)
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

type kubectlDiffObject struct {
	name    string
	added   int
	removed int
}

type kubectlDiffScan struct {
	objects []kubectlDiffObject
	alerts  []string
}

func (s *kubectlDiffScan) ingestLine(line string) {
	switch {
	case strings.HasPrefix(line, "diff "):
		s.objects = append(s.objects, kubectlDiffObject{name: kubectlDiffObjectName(line)})
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
	case strings.HasPrefix(line, "+"):
		if len(s.objects) > 0 {
			s.objects[len(s.objects)-1].added++
		}
	case strings.HasPrefix(line, "-"):
		if len(s.objects) > 0 {
			s.objects[len(s.objects)-1].removed++
		}
	default:
		s.ingestAlertCandidate(line)
	}
}

func (s *kubectlDiffScan) ingestAlertCandidate(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed != "" && isKubectlAlertLine(trimmed) {
		s.alerts = append(s.alerts, shared.Clip(trimmed, 160))
	}
}

func kubectlDiffObjectName(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return path.Base(fields[len(fields)-1])
}

func renderKubectlDiffScan(scan *kubectlDiffScan, maxLines int) kubernetesSummaryResult {
	out := []string{fmt.Sprintf("diff: %d objects changed", len(scan.objects))}
	out = append(out, scan.alerts...)
	for _, object := range scan.objects {
		out = append(out, shared.Clip(fmt.Sprintf("%s: +%d -%d", object.name, object.added, object.removed), 160))
	}
	return summarizeKubernetesLines(out, maxLines)
}

func summarizeDiffResult(input string, maxLines int) kubernetesSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	scan := &kubectlDiffScan{}
	for _, line := range strings.Split(clean, "\n") {
		scan.ingestLine(line)
	}

	scan.alerts = shared.UniqueStrings(scan.alerts)
	if len(scan.objects) == 0 {
		if len(scan.alerts) > 0 {
			return summarizeKubernetesLines(scan.alerts, maxLines)
		}
		return kubernetesSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	return renderKubectlDiffScan(scan, maxLines)
}
