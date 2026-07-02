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

func summarizeTerraformResult(input string, maxLines int) buildSystemSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	const attrCap = 4
	headerCap := maxLines / 2
	if headerCap < 3 {
		headerCap = 3
	}

	headers := []string{}
	attrs := []string{}
	summaries := []string{}
	alerts := []string{}
	alertContext := 0
	for _, raw := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(raw), "│╷╵┃"))
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "Error:") || strings.HasPrefix(trimmed, "Warning:"):
			alerts = append(alerts, shared.Clip(trimmed, 160))
			alertContext = 4
		case alertContext > 0:
			alerts = append(alerts, "  "+shared.Clip(trimmed, 158))
			alertContext--
		case isTerraformResourceHeader(trimmed):
			headers = append(headers, shared.Clip(trimmed, 160))
		case isTerraformSummaryLine(trimmed):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		case isTerraformAttrLine(trimmed):
			attrs = append(attrs, shared.Clip(trimmed, 160))
		}
	}

	summaries = shared.UniqueStrings(summaries)
	if len(headers) == 0 && len(attrs) == 0 && len(summaries) == 0 && len(alerts) == 0 {
		return buildSystemSummaryResult{Text: shared.CompactLines(input, maxLines)}
	}

	out := append([]string{}, summaries...)
	out = append(out, alerts...)
	if len(headers) > headerCap {
		out = append(out, headers[:headerCap]...)
		out = append(out, fmt.Sprintf("+%d more resource changes", len(headers)-headerCap))
	} else {
		out = append(out, headers...)
	}
	if len(attrs) > attrCap {
		out = append(out, attrs[:attrCap]...)
		out = append(out, fmt.Sprintf("+%d more attribute lines", len(attrs)-attrCap))
	} else {
		out = append(out, attrs...)
	}

	result := buildSystemSummaryResult{
		Text: shared.JoinLimitedLines(out, maxLines),
	}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
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
