package dotnet

import (
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

type dotnetSummaryResult struct {
	Text         string
	OmittedCount int
}

func SummarizeDotnetBuild(input string, maxLines int) string {
	return summarizeDotnetBuildResult(input, maxLines).Text
}

func DotnetBuildRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDotnetBuildResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

// summarizeDotnetBuildResult keeps MSBuild diagnostics with their CS/MSB/NU
// codes and file(line,col) anchors ahead of restore and progress chatter.
// MSBuild repeats every diagnostic in its closing summary, so entries are
// deduplicated before rendering.
func summarizeDotnetBuildResult(input string, maxLines int) dotnetSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	errors, warnings, summaries := collectDotnetBuildLines(clean)
	if len(errors) == 0 && len(warnings) == 0 {
		if len(summaries) > 0 {
			return summarizeDotnetLines(summaries, maxLines)
		}
		return dotnetSummaryResult{Text: shared.SummarizeGenericFailure(clean, maxLines)}
	}
	return composeDotnetBuild(errors, warnings, summaries, maxLines)
}

func collectDotnetBuildLines(clean string) (errors, warnings, summaries []string) {
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		switch {
		case isDotnetDiagnosticLine(trimmed, "error"):
			errors = append(errors, shared.Clip(trimmed, 200))
		case isDotnetDiagnosticLine(trimmed, "warning"):
			warnings = append(warnings, shared.Clip(trimmed, 200))
		case isDotnetBuildSummaryLine(trimmed):
			summaries = append(summaries, shared.Clip(trimmed, 160))
		}
	}
	errors = shared.UniqueStrings(shared.FoldConsecutiveLines(errors))
	warnings = shared.UniqueStrings(shared.FoldConsecutiveLines(warnings))
	summaries = prioritizeDotnetBuildSummaries(shared.UniqueStrings(shared.FoldConsecutiveLines(summaries)))
	return errors, warnings, summaries
}

// composeDotnetBuild keeps every error, then warnings within budget, then
// the prioritized build summary lines.
func composeDotnetBuild(errors, warnings, summaries []string, maxLines int) dotnetSummaryResult {
	budget := maxLines - minInt(len(summaries), 2)
	if budget < len(errors) {
		budget = len(errors)
	}

	out := appendWithinLimit(append([]string{}, errors...), warnings, budget)
	out = appendWithinLimit(out, summaries, maxLines)
	return dotnetResultWithOmitted(out, len(errors)+len(warnings)+len(summaries), maxLines)
}

// appendWithinLimit appends lines until out reaches limit.
func appendWithinLimit(out, lines []string, limit int) []string {
	for _, line := range lines {
		if len(out) >= limit {
			break
		}
		out = append(out, line)
	}
	return out
}

func dotnetResultWithOmitted(out []string, total, maxLines int) dotnetSummaryResult {
	result := dotnetSummaryResult{Text: shared.JoinLimitedLines(out, maxLines)}
	if total > len(out) {
		result.OmittedCount = total - len(out)
	}
	return result
}

func SummarizeDotnetTest(input string, maxLines int) string {
	return summarizeDotnetTestResult(input, maxLines).Text
}

func DotnetTestRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDotnetTestResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

// dotnetTestBlock groups one VSTest failure: the "Failed <TestName>" entry,
// its Error Message lines (Assert output, Expected/Actual), and the first
// stack-trace anchor.
type dotnetTestBlock struct {
	name     string
	messages []string
	stack    string
}

func summarizeDotnetTestResult(input string, maxLines int) dotnetSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	blocks, buildErrors, summaries := collectDotnetTestOutput(clean)
	if len(blocks) == 0 && len(buildErrors) == 0 {
		if len(summaries) > 0 {
			return summarizeDotnetLines(summaries, maxLines)
		}
		return dotnetSummaryResult{Text: shared.SummarizeGenericFailure(clean, maxLines)}
	}
	return renderDotnetTestBlocks(blocks, buildErrors, summaries, maxLines)
}

type dotnetTestParser struct {
	blocks      []dotnetTestBlock
	buildErrors []string
	summaries   []string
	state       int
}

const (
	dotnetStateNone = iota
	dotnetStateMessage
	dotnetStateStack
)

func collectDotnetTestOutput(clean string) ([]dotnetTestBlock, []string, []string) {
	parser := dotnetTestParser{}
	for _, line := range shared.NonEmptyLines(clean) {
		parser.consume(strings.TrimSpace(line))
	}
	buildErrors := shared.UniqueStrings(shared.FoldConsecutiveLines(parser.buildErrors))
	summaries := shared.UniqueStrings(shared.FoldConsecutiveLines(parser.summaries))
	return parser.blocks, buildErrors, summaries
}

func (p *dotnetTestParser) consume(trimmed string) {
	switch {
	case isDotnetFailedTestLine(trimmed):
		p.blocks = append(p.blocks, dotnetTestBlock{name: shared.Clip(trimmed, 160)})
		p.state = dotnetStateNone
	case isDotnetTestSummaryLine(trimmed):
		p.summaries = append(p.summaries, shared.Clip(trimmed, 200))
		p.state = dotnetStateNone
	case isDotnetDiagnosticLine(trimmed, "error"):
		p.buildErrors = append(p.buildErrors, shared.Clip(trimmed, 200))
		p.state = dotnetStateNone
	case trimmed == "Error Message:":
		p.state = dotnetStateMessage
	case trimmed == "Stack Trace:":
		p.state = dotnetStateStack
	default:
		p.consumeBlockDetail(trimmed)
	}
}

func (p *dotnetTestParser) consumeBlockDetail(trimmed string) {
	if len(p.blocks) == 0 {
		return
	}
	block := &p.blocks[len(p.blocks)-1]
	switch {
	case p.state == dotnetStateMessage && len(block.messages) < 4:
		block.messages = append(block.messages, shared.Clip(trimmed, 160))
	case p.state == dotnetStateStack && block.stack == "" && strings.HasPrefix(trimmed, "at "):
		block.stack = shared.Clip(trimmed, 160)
	}
}

// renderDotnetTestBlocks emits grouped failures with tiered inclusion: every
// failing test name always survives, then assertion messages and stack
// anchors fill the remaining budget, followed by the run summary.
func renderDotnetTestBlocks(blocks []dotnetTestBlock, buildErrors, summaries []string, maxLines int) dotnetSummaryResult {
	total := len(buildErrors) + len(summaries) + normalizeDotnetTestBlocks(blocks)
	budget := maxLines - minInt(len(summaries), 1)
	if budget < len(blocks)+len(buildErrors) {
		budget = len(blocks) + len(buildErrors)
	}

	include := selectDotnetTestLines(blocks, len(buildErrors), budget)
	out := append([]string{}, buildErrors...)
	for i := range blocks {
		out = append(out, include[i]...)
	}
	out = appendWithinLimit(out, summaries, maxLines)
	return dotnetResultWithOmitted(out, total, maxLines)
}

func normalizeDotnetTestBlocks(blocks []dotnetTestBlock) int {
	total := 0
	for i := range blocks {
		blocks[i].messages = shared.UniqueStrings(shared.FoldConsecutiveLines(blocks[i].messages))
		total += 1 + len(blocks[i].messages)
		if blocks[i].stack != "" {
			total++
		}
	}
	return total
}

// selectDotnetTestLines picks grouped lines per failure in priority tiers so
// every failing test name survives before any block gets its message lines
// or stack anchor.
func selectDotnetTestLines(blocks []dotnetTestBlock, used, budget int) [][]string {
	include := make([][]string, len(blocks))
	for i := range blocks {
		include[i] = []string{blocks[i].name}
		used++
	}
	used = appendDotnetMessages(blocks, include, used, budget)
	appendDotnetStacks(blocks, include, used, budget)
	return include
}

func appendDotnetMessages(blocks []dotnetTestBlock, include [][]string, used, budget int) int {
	for pass := 0; pass < 4; pass++ {
		for i := range blocks {
			if used >= budget {
				return used
			}
			if pass < len(blocks[i].messages) {
				include[i] = append(include[i], blocks[i].messages[pass])
				used++
			}
		}
	}
	return used
}

func appendDotnetStacks(blocks []dotnetTestBlock, include [][]string, used, budget int) {
	for i := range blocks {
		if used >= budget {
			return
		}
		if blocks[i].stack != "" {
			include[i] = append(include[i], blocks[i].stack)
			used++
		}
	}
}

// isDotnetDiagnosticLine matches MSBuild diagnostics in both the
// "Path(48,17): error CS0103: message" and "MSBUILD : error MSB1009: ..."
// shapes, requiring a real diagnostic code after the severity.
func isDotnetDiagnosticLine(line, severity string) bool {
	rest := ""
	switch {
	case strings.HasPrefix(line, severity+" "):
		rest = line[len(severity)+1:]
	default:
		idx := strings.Index(line, ": "+severity+" ")
		if idx < 0 {
			return false
		}
		rest = line[idx+len(severity)+3:]
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return false
	}
	return isDotnetDiagnosticCode(strings.TrimSuffix(fields[0], ":"))
}

// isDotnetDiagnosticCode reports whether a token looks like CS1002, MSB3073,
// NETSDK1045, NU1101, or xUnit1013: a letter prefix followed by digits.
func isDotnetDiagnosticCode(token string) bool {
	letters := 0
	for letters < len(token) && isASCIILetter(token[letters]) {
		letters++
	}
	if letters < 2 || letters == len(token) {
		return false
	}
	for _, r := range token[letters:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isASCIILetter(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func isDotnetBuildSummaryLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "Build FAILED"),
		strings.HasPrefix(line, "Build succeeded"),
		strings.HasSuffix(line, "Warning(s)"),
		strings.HasSuffix(line, "Error(s)"),
		strings.HasPrefix(line, "Time Elapsed"):
		return true
	default:
		return false
	}
}

func prioritizeDotnetBuildSummaries(lines []string) []string {
	prioritized := []string{}
	for _, check := range []func(string) bool{
		func(line string) bool { return strings.HasPrefix(line, "Build FAILED") },
		func(line string) bool { return strings.HasPrefix(line, "Build succeeded") },
		func(line string) bool { return strings.HasSuffix(line, "Error(s)") },
		func(line string) bool { return strings.HasSuffix(line, "Warning(s)") },
		func(line string) bool { return strings.HasPrefix(line, "Time Elapsed") },
	} {
		for _, line := range lines {
			if check(line) {
				prioritized = append(prioritized, line)
			}
		}
	}
	if len(prioritized) == 0 {
		return lines
	}
	return shared.UniqueStrings(prioritized)
}

// isDotnetFailedTestLine matches VSTest per-test failure entries like
// "Failed Acme.Tests.OrderTests.Applies_Discount [4 ms]".
func isDotnetFailedTestLine(line string) bool {
	if !strings.HasPrefix(line, "Failed ") {
		return false
	}
	rest := strings.TrimPrefix(line, "Failed ")
	return rest != "" && !strings.HasPrefix(rest, "-")
}

func isDotnetTestSummaryLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "Failed!"),
		strings.HasPrefix(line, "Passed!"),
		strings.HasPrefix(line, "Test Run Failed"),
		strings.HasPrefix(line, "Test Run Successful"),
		strings.HasPrefix(line, "Total tests:"):
		return true
	default:
		return false
	}
}

func summarizeDotnetLines(lines []string, maxLines int) dotnetSummaryResult {
	result := dotnetSummaryResult{Text: shared.JoinLimitedLines(lines, maxLines)}
	if len(lines) > maxLines {
		result.OmittedCount = len(lines) - maxLines
	}
	return result
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
