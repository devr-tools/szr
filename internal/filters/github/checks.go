package github

import (
	"fmt"
	"regexp"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizePRChecks(input string, maxLines int) string {
	return summarizePRChecksResult(input, maxLines).Text
}

func SummarizeGHPRChecks(input string, maxLines int) string {
	return SummarizePRChecks(input, maxLines)
}

func GHPRChecksRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizePRChecksResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type checkRow struct {
	name    string
	status  string
	elapsed string
	url     string
}

type checksScan struct {
	rows       map[string]checkRow
	order      []string
	seen       map[string]int
	extras     []string
	duplicates int
}

var checksStatusOrder = []string{"pass", "fail", "pending", "skipping", "cancelled", "neutral"}

var checksStatusWords = map[string]string{
	"pass": "pass", "ok": "pass", "success": "pass", "successful": "pass",
	"fail": "fail", "failure": "fail", "failing": "fail", "error": "fail",
	"pending": "pending", "queued": "pending", "in_progress": "pending", "running": "pending", "waiting": "pending",
	"skipping": "skipping", "skipped": "skipping",
	"cancelled": "cancelled", "canceled": "cancelled",
	"neutral": "neutral",
}

var checksStatusSymbols = map[string]string{
	"✓": "pass", "√": "pass",
	"x": "fail", "✗": "fail", "✘": "fail",
	"*": "pending",
	"-": "skipping",
}

var checksColumnSplitter = regexp.MustCompile(`\s{2,}`)

func newChecksScan() *checksScan {
	return &checksScan{rows: map[string]checkRow{}, seen: map[string]int{}}
}

func (s *checksScan) ingestLine(line string) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || isChecksChromeLine(trimmed) {
		return
	}
	row, ok := parseCheckRow(trimmed)
	if !ok {
		s.extras = append(s.extras, shared.Clip(trimmed, 160))
		return
	}
	if _, exists := s.rows[row.name]; !exists {
		s.order = append(s.order, row.name)
	} else {
		s.duplicates++
	}
	s.rows[row.name] = row
	s.seen[row.name]++
}

// watchedUpdates reports how many table repaints beyond the first render were
// observed, using the most-repeated check name as the render count.
func (s *checksScan) watchedUpdates() int {
	renders := 0
	for _, count := range s.seen {
		if count > renders {
			renders = count
		}
	}
	if renders <= 1 {
		return 0
	}
	return renders - 1
}

func (s *checksScan) statusCounts() map[string]int {
	counts := map[string]int{}
	for _, name := range s.order {
		counts[s.rows[name].status]++
	}
	return counts
}

func parseCheckRow(line string) (checkRow, bool) {
	fields := splitCheckFields(line)
	if len(fields) < 2 {
		return checkRow{}, false
	}
	if status, ok := checksStatusSymbols[strings.ToLower(fields[0])]; ok {
		return buildCheckRow(fields[1], status, fields[2:]), true
	}
	for i := 1; i < len(fields); i++ {
		status, ok := checksStatusWords[strings.ToLower(fields[i])]
		if !ok {
			continue
		}
		return buildCheckRow(strings.Join(fields[:i], " "), status, fields[i+1:]), true
	}
	return checkRow{}, false
}

func splitCheckFields(line string) []string {
	var raw []string
	if strings.Contains(line, "\t") {
		raw = strings.Split(line, "\t")
	} else {
		raw = checksColumnSplitter.Split(line, -1)
	}
	fields := make([]string, 0, len(raw))
	for _, field := range raw {
		field = strings.TrimSpace(field)
		if field != "" {
			fields = append(fields, field)
		}
	}
	if len(fields) < 2 {
		return strings.Fields(line)
	}
	return fields
}

func buildCheckRow(name, status string, rest []string) checkRow {
	row := checkRow{name: name, status: status}
	for _, field := range rest {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			if row.url == "" {
				row.url = field
			}
			continue
		}
		if row.elapsed == "" {
			row.elapsed = field
		}
	}
	return row
}

func isChecksChromeLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "refreshing checks status") || strings.Contains(lower, "press ctrl+c") {
		return true
	}
	if strings.Contains(lower, "all checks were successful") || strings.Contains(lower, "checks were not successful") {
		return true
	}
	return strings.Contains(lower, "failing") && strings.Contains(lower, "successful") && strings.Contains(lower, "checks")
}

func renderChecksHeader(scan *checksScan) string {
	counts := scan.statusCounts()
	parts := make([]string, 0, len(checksStatusOrder))
	for _, status := range checksStatusOrder {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	header := fmt.Sprintf("checks: %s (%d total)", strings.Join(parts, ", "), len(scan.order))
	if updates := scan.watchedUpdates(); updates > 0 {
		header += fmt.Sprintf(" (watched %d updates)", updates)
	}
	return header
}

// renderActionableChecks lists failing, cancelled, and pending checks with
// their full URL, which is the actionable link; passing and skipped checks
// stay in the header counts only.
func renderActionableChecks(scan *checksScan) []string {
	out := []string{}
	for _, status := range []string{"fail", "cancelled", "pending"} {
		for _, name := range scan.order {
			row := scan.rows[name]
			if row.status != status {
				continue
			}
			parts := []string{row.status + ":", row.name}
			if row.elapsed != "" && row.elapsed != "0" {
				parts = append(parts, row.elapsed)
			}
			if row.url != "" {
				parts = append(parts, row.url)
			}
			out = append(out, shared.Clip(strings.Join(parts, " "), 200))
		}
	}
	return out
}

func renderChecksScan(scan *checksScan, maxLines int) githubSummaryResult {
	out := []string{renderChecksHeader(scan)}
	listed := renderActionableChecks(scan)
	out = append(out, listed...)
	out = append(out, shared.UniqueStrings(scan.extras)...)
	result := summarizeGithubLines(out, maxLines)
	result.OmittedCount += scan.duplicates + len(scan.order) - len(listed)
	return result
}

func summarizePRChecksResult(input string, maxLines int) githubSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return githubSummaryResult{Text: "ok"}
	}
	scan := newChecksScan()
	for _, line := range lines {
		scan.ingestLine(line)
	}
	if len(scan.order) == 0 {
		return githubSummaryResult{Text: shared.SummarizeGenericFailure(clean, maxLines)}
	}
	return renderChecksScan(scan, maxLines)
}
