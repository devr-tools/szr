package gitlab

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

const keptDominantRows = 3

var (
	pipelineRowPattern = regexp.MustCompile(`^\(([A-Za-z_]+)\)\s*[•·]?\s*(.+)$`)
	pipelineAgePattern = regexp.MustCompile(`\(([^()]* ago)\)`)
)

type pipelineRow struct {
	status string
	detail string
}

type pipelineSummaryResult struct {
	Text         string
	OmittedCount int
}

func SummarizePipelines(input string, maxLines int) string {
	return summarizePipelinesResult(input, maxLines).Text
}

func PipelineRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizePipelinesResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional rows", result.OmittedCount))
}

func summarizePipelinesResult(input string, maxLines int) pipelineSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := strings.TrimSpace(shared.StripANSI(input))
	rows, context := parsePipelineLines(clean)
	if len(rows) == 0 {
		return pipelineSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	kept, omitted := selectPipelineRows(rows)
	out := append([]string{pipelineHeader(rows)}, kept...)
	if omitted > 0 {
		out = append(out, fmt.Sprintf("... +%d more %s", omitted, dominantPipelineStatus(rows)))
	}
	out = append(out, context...)
	result := pipelineSummaryResult{Text: shared.JoinLimitedLines(out, maxLines), OmittedCount: omitted}
	if len(out) > maxLines {
		result.OmittedCount += len(out) - maxLines
	}
	return result
}

// parsePipelineLines splits glab output into status-marked rows such as
// `(failed) • #418223 feat/x (about 3 hours ago)` and the few context lines
// worth keeping (those naming a pipeline or job id).
func parsePipelineLines(input string) ([]pipelineRow, []string) {
	rows := []pipelineRow{}
	context := []string{}
	for _, line := range shared.NonEmptyLines(input) {
		if match := pipelineRowPattern.FindStringSubmatch(line); match != nil {
			rows = append(rows, pipelineRow{
				status: strings.ToLower(match[1]),
				detail: collapseSpaces(match[2]),
			})
			continue
		}
		if strings.Contains(line, "#") && len(context) < 2 {
			context = append(context, shared.Clip(collapseSpaces(line), 160))
		}
	}
	return rows, context
}

func collapseSpaces(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func pipelineHeader(rows []pipelineRow) string {
	counts, order := pipelineStatusCounts(rows)
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	parts := make([]string, 0, len(order))
	for _, status := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
	}
	return fmt.Sprintf("pipelines: %d (%s)", len(rows), strings.Join(parts, " "))
}

func pipelineStatusCounts(rows []pipelineRow) (map[string]int, []string) {
	counts := map[string]int{}
	order := []string{}
	for _, row := range rows {
		if _, seen := counts[row.status]; !seen {
			order = append(order, row.status)
		}
		counts[row.status]++
	}
	return counts, order
}

func dominantPipelineStatus(rows []pipelineRow) string {
	counts, order := pipelineStatusCounts(rows)
	dominant := ""
	for _, status := range order {
		if dominant == "" || counts[status] > counts[dominant] {
			dominant = status
		}
	}
	return dominant
}

// selectPipelineRows keeps every non-dominant row (the failed or still-moving
// entries are the payload) plus a few leading dominant rows for recency.
func selectPipelineRows(rows []pipelineRow) ([]string, int) {
	dominant := dominantPipelineStatus(rows)
	out := []string{}
	keptDominant := 0
	omitted := 0
	for _, row := range rows {
		if row.status == dominant {
			if keptDominant >= keptDominantRows {
				omitted++
				continue
			}
			keptDominant++
		}
		out = append(out, shared.Clip(row.status+": "+compactPipelineAges(row.detail), 160))
	}
	return out, omitted
}

func compactPipelineAges(detail string) string {
	return pipelineAgePattern.ReplaceAllStringFunc(detail, func(match string) string {
		return "(" + shared.CompactRelativeAge(strings.Trim(match, "()")) + ")"
	})
}
