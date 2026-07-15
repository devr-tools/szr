package pulumi

import (
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizePulumi(input string, maxLines int) string {
	return SummarizePulumiUnderContract(input, maxLines, false)
}

// SummarizePulumiUnderContract renders the pulumi summary; when contract is
// true the render self-caps to the predicted engine compression-contract
// allowance so diagnostics and resource counts survive downstream verbatim.
func SummarizePulumiUnderContract(input string, maxLines int, contract bool) string {
	return summarizePulumiResult(input, maxLines, contract).Text
}

func PulumiRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return PulumiRecoveryInfoUnderContract(input, maxLines, false)
}

// PulumiRecoveryInfoUnderContract mirrors SummarizePulumiUnderContract for
// the recovery plan.
func PulumiRecoveryInfoUnderContract(input string, maxLines int, contract bool) (string, string, bool) {
	result := summarizePulumiResult(input, maxLines, contract)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type pulumiSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizePulumiResult(input string, maxLines int, contract bool) pulumiSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	scan := scanPulumiLines(clean)
	candidates := scan.priorityLines()
	if len(candidates) == 0 {
		return pulumiSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	allowance := 0
	if contract {
		allowance = shared.PredictedTokenAllowance(input, maxLines)
	}
	selected, omitted := shared.FitPriorityLines(candidates, maxLines, allowance)
	return pulumiSummaryResult{
		Text:         strings.Join(selected, "\n"),
		OmittedCount: omitted + scan.unchanged,
	}
}

func scanPulumiLines(clean string) *pulumiScan {
	scan := &pulumiScan{}
	for _, line := range shared.NonEmptyLines(clean) {
		scan.ingestLine(strings.TrimSpace(line))
	}
	return scan
}

// Selection tiers for pulumi output: diagnostics, errors, resource counts,
// and the duration line are irreducible; per-resource change rows explain the
// diff; the operation header and stack outputs provide context.
const (
	pulumiTierCritical = iota
	pulumiTierChange
	pulumiTierContext
)

// pulumiScan classifies pulumi output line by line, tracking the active
// section so diagnostics and summary blocks are kept in full while unchanged
// resource-table rows are only counted.
type pulumiScan struct {
	lines     []shared.PriorityLine
	section   string
	unchanged int
}

func (s *pulumiScan) ingestLine(trimmed string) {
	switch {
	case isPulumiHeaderLine(trimmed):
		s.section = ""
		s.appendLine(trimmed, pulumiTierContext)
	case trimmed == "Diagnostics:", trimmed == "Resources:":
		s.section = trimmed
		s.appendLine(trimmed, pulumiTierCritical)
	case trimmed == "Outputs:":
		s.section = trimmed
		s.appendLine(trimmed, pulumiTierContext)
	case strings.HasPrefix(trimmed, "Duration:"):
		s.section = ""
		s.appendLine(trimmed, pulumiTierCritical)
	case s.section != "":
		s.ingestSectionLine(trimmed)
	default:
		s.ingestTableLine(trimmed)
	}
}

func (s *pulumiScan) ingestSectionLine(trimmed string) {
	switch s.section {
	case "Diagnostics:":
		s.appendLine(trimmed, pulumiTierCritical)
	case "Resources:":
		s.appendLine(trimmed, pulumiTierCritical)
	case "Outputs:":
		s.appendLine(trimmed, pulumiTierContext)
	}
}

// ingestTableLine handles the resource-diff table: rows led by an operation
// symbol are kept, unchanged rows are only counted, and errors that surface
// outside the diagnostics section stay irreducible.
func (s *pulumiScan) ingestTableLine(trimmed string) {
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return
	}
	switch {
	case isPulumiOpSymbol(fields[0]):
		s.appendLine(strings.Join(dropTreeGlyphs(fields), " "), pulumiTierChange)
	case isPulumiUnchangedRow(fields):
		s.unchanged++
	case strings.Contains(trimmed, "error:"):
		s.appendLine(trimmed, pulumiTierCritical)
	}
}

func (s *pulumiScan) appendLine(trimmed string, tier int) {
	s.lines = append(s.lines, shared.PriorityLine{Text: shared.Clip(trimmed, 160), Tier: tier})
}

func (s *pulumiScan) priorityLines() []shared.PriorityLine {
	if len(s.lines) == 0 {
		return nil
	}
	return s.lines
}

func isPulumiHeaderLine(trimmed string) bool {
	for _, prefix := range []string{"Previewing ", "Updating ", "Destroying ", "Refreshing "} {
		if strings.HasPrefix(trimmed, prefix) && strings.HasSuffix(trimmed, ":") {
			return true
		}
	}
	return false
}

func isPulumiOpSymbol(field string) bool {
	switch field {
	case "+", "-", "~", "+-", "-+", "++", "--":
		return true
	default:
		return false
	}
}

// isPulumiUnchangedRow recognizes resource-table rows without an operation
// symbol: a tree glyph or a provider:module:Type token leads the row.
func isPulumiUnchangedRow(fields []string) bool {
	head := fields[0]
	if head == "├─" || head == "└─" {
		return true
	}
	return strings.Count(head, ":") >= 2 && !strings.HasSuffix(head, ":")
}

func dropTreeGlyphs(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "├─" || field == "└─" || field == "│" {
			continue
		}
		out = append(out, field)
	}
	return out
}
