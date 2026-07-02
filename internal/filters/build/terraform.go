package build

import (
	"fmt"
	"regexp"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var terraformMarkers = []string{
	"Terraform will perform the following actions",
	"OpenTofu will perform the following actions",
	"Terraform used the selected providers",
	"OpenTofu used the selected providers",
	"Terraform planned the following actions",
	"OpenTofu planned the following actions",
	"Terraform has been successfully initialized",
	"OpenTofu has been successfully initialized",
	"No changes. Your infrastructure matches the configuration.",
	"Apply complete!",
	"Destroy complete!",
	"Planning failed.",
	"│ Error:",
	"│ Warning:",
}

var terraformPlanSummaryPattern = regexp.MustCompile(`Plan: \d+ to add, \d+ to change, \d+ to destroy`)

func isTerraformOutput(input string) bool {
	for _, marker := range terraformMarkers {
		if strings.Contains(input, marker) {
			return true
		}
	}
	return terraformPlanSummaryPattern.MatchString(input)
}

func SummarizeTerraform(input string, maxLines int) string {
	return summarizeTerraformResult(shared.StripANSI(input), maxLines).Text
}

type terraformScan struct {
	headers      []string
	attrs        []string
	summaries    []string
	alerts       []string
	alertContext int
}

func (s *terraformScan) ingestLine(raw string) {
	trimmed := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(raw), "│╷╵┃"))
	if trimmed == "" {
		return
	}
	switch {
	case strings.HasPrefix(trimmed, "Error:") || strings.HasPrefix(trimmed, "Warning:"):
		s.alerts = append(s.alerts, shared.Clip(trimmed, 160))
		s.alertContext = 4
	case s.alertContext > 0:
		s.alerts = append(s.alerts, "  "+shared.Clip(trimmed, 158))
		s.alertContext--
	case isTerraformResourceHeader(trimmed):
		s.headers = append(s.headers, shared.Clip(trimmed, 160))
	case isTerraformSummaryLine(trimmed):
		s.summaries = append(s.summaries, shared.Clip(trimmed, 160))
	case isTerraformAttrLine(trimmed):
		s.attrs = append(s.attrs, shared.Clip(trimmed, 160))
	}
}

func (s *terraformScan) empty() bool {
	return len(s.headers) == 0 && len(s.attrs) == 0 && len(s.summaries) == 0 && len(s.alerts) == 0
}

func appendTerraformCappedLines(out, lines []string, limit int, moreFormat string) []string {
	if len(lines) > limit {
		out = append(out, lines[:limit]...)
		return append(out, fmt.Sprintf(moreFormat, len(lines)-limit))
	}
	return append(out, lines...)
}

func renderTerraformScan(scan *terraformScan, maxLines int) buildSystemSummaryResult {
	const attrCap = 4
	headerCap := maxLines / 2
	if headerCap < 3 {
		headerCap = 3
	}

	out := append([]string{}, scan.summaries...)
	out = append(out, scan.alerts...)
	out = appendTerraformCappedLines(out, scan.headers, headerCap, "+%d more resource changes")
	out = appendTerraformCappedLines(out, scan.attrs, attrCap, "+%d more attribute lines")

	result := buildSystemSummaryResult{
		Text: shared.JoinLimitedLines(out, maxLines),
	}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}

func summarizeTerraformResult(input string, maxLines int) buildSystemSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	scan := &terraformScan{}
	for _, raw := range strings.Split(input, "\n") {
		scan.ingestLine(raw)
	}

	scan.summaries = shared.UniqueStrings(scan.summaries)
	if scan.empty() {
		return buildSystemSummaryResult{Text: shared.CompactLines(input, maxLines)}
	}
	return renderTerraformScan(scan, maxLines)
}

func isTerraformResourceHeader(line string) bool {
	if !strings.HasPrefix(line, "# ") {
		return false
	}
	return strings.Contains(line, " will be ") ||
		strings.Contains(line, " must be ") ||
		strings.Contains(line, " has moved to ")
}

func isTerraformSummaryLine(line string) bool {
	for _, prefix := range []string{
		"Plan:",
		"Apply complete!",
		"Destroy complete!",
		"No changes.",
		"Changes to Outputs:",
		"Planning failed.",
		"Terraform has been successfully initialized",
		"OpenTofu has been successfully initialized",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isTerraformAttrLine(line string) bool {
	for _, prefix := range []string{"+ ", "- ", "~ ", "-/+ ", "+/- ", "<= "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}
