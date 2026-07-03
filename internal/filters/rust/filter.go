package rust

import (
	"strconv"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeCargoTest(input string, maxLines int) string {
	return summarizeCargoTestResult(input, maxLines).Text
}

func CargoTestRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeCargoTestResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

type rustSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeCargoTestResult(input string, maxLines int) rustSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return rustSummaryResult{Text: "ok"}
	}

	summaries := []string{}
	resultLine := ""
	rerunLine := ""
	failures := []string{}
	details := []string{}
	inFailureList := false
	for _, line := range lines {
		inFailureList = collectCargoTestLine(
			strings.TrimSpace(line),
			&summaries,
			&resultLine,
			&rerunLine,
			&failures,
			&details,
			inFailureList,
		)
	}

	summaries = uniqueStrings(shared.FoldConsecutiveLines(summaries))
	failures = uniqueStrings(shared.FoldConsecutiveLines(failures))
	details = uniqueStrings(shared.FoldConsecutiveLines(details))

	return renderCargoTestSummary(input, summaries, failures, details, resultLine, rerunLine, maxLines)
}

func collectCargoTestLine(
	trimmed string,
	summaries *[]string,
	resultLine *string,
	rerunLine *string,
	failures *[]string,
	details *[]string,
	inFailureList bool,
) bool {
	switch {
	case strings.HasPrefix(trimmed, "running "):
		*summaries = append(*summaries, clip(trimmed, 160))
	case strings.HasPrefix(trimmed, "test result:"):
		*resultLine = clip(trimmed, 160)
		*summaries = append(*summaries, *resultLine)
	case strings.HasPrefix(trimmed, "error: test failed"):
		*rerunLine = clip(trimmed, 160)
		*summaries = append(*summaries, *rerunLine)
	case strings.HasPrefix(trimmed, "Finished `test`"):
		*summaries = append(*summaries, clip(trimmed, 160))
	case strings.HasPrefix(trimmed, "failures:"):
		return true
	case strings.HasPrefix(trimmed, "test ") && strings.Contains(trimmed, "FAILED"):
		*failures = append(*failures, clip(trimmed, 160))
	case isCargoDiagnosticHeader(trimmed):
		*failures = append(*failures, clip(trimmed, 160))
	case inFailureList && isCargoFailureName(trimmed):
		*failures = append(*failures, clip(trimmed, 160))
	case isCargoInterestingDetail(trimmed):
		*details = append(*details, clip(trimmed, 160))
	default:
		if trimmed == "" || isDividerLine(trimmed) {
			return inFailureList
		}
	}
	return false
}

func renderCargoTestSummary(
	input string,
	summaries []string,
	failures []string,
	details []string,
	resultLine string,
	rerunLine string,
	maxLines int,
) rustSummaryResult {
	if len(failures) == 0 && len(details) == 0 {
		return fallbackCargoTestSummary(input, summaries, resultLine, rerunLine, maxLines)
	}

	stackDetails, hints, rootDetails := splitCargoDetails(details)
	failures = pruneCargoFailureNames(failures)
	out := append([]string{}, failures...)
	out = append(out, rootDetails...)
	out = append(out, shared.SelectUniqueAnchoredLines(stackDetails, maxLines/3+1)...)
	if resultLine != "" {
		out = append(out, resultLine)
	}
	if rerunLine != "" {
		out = append(out, rerunLine)
	}
	out = append(out, hints...)
	return summarizeRustLines(out, maxLines)
}

func fallbackCargoTestSummary(input string, summaries []string, resultLine string, rerunLine string, maxLines int) rustSummaryResult {
	if resultLine != "" {
		return rustSummaryResult{Text: resultLine}
	}
	if rerunLine != "" {
		return rustSummaryResult{Text: rerunLine}
	}
	if len(summaries) > 0 {
		return summarizeRustLines(summaries, maxLines)
	}
	return rustSummaryResult{Text: shared.CompactLines(input, maxLines)}
}

func splitCargoDetails(details []string) ([]string, []string, []string) {
	stackDetails := []string{}
	hints := []string{}
	rootDetails := []string{}
	for _, line := range details {
		switch {
		case strings.HasPrefix(line, "= help:"), strings.HasPrefix(line, "= note:"), strings.HasPrefix(line, "help:"), strings.HasPrefix(line, "note:"):
			hints = append(hints, line)
		case strings.HasPrefix(line, "thread '"), strings.Contains(line, " panicked at "):
			rootDetails = append(rootDetails, line)
		case shared.DiagnosticAnchor(line) != "" || strings.HasPrefix(line, "--> "):
			stackDetails = append(stackDetails, line)
		default:
			rootDetails = append(rootDetails, line)
		}
	}
	return stackDetails, hints, rootDetails
}

func SummarizeCargoBuild(input string, maxLines int) string {
	return SummarizeCargoBuildUnderContract(input, maxLines, false)
}

// SummarizeCargoBuildUnderContract renders the build/clippy summary; when
// contract is true the render self-caps to the predicted engine
// compression-contract allowance so every diagnostic header and lint slug
// survives downstream verbatim.
func SummarizeCargoBuildUnderContract(input string, maxLines int, contract bool) string {
	return summarizeCargoBuildResult(input, maxLines, contract).Text
}

func CargoBuildRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return CargoBuildRecoveryInfoUnderContract(input, maxLines, false)
}

// CargoBuildRecoveryInfoUnderContract mirrors SummarizeCargoBuildUnderContract
// for the recovery plan.
func CargoBuildRecoveryInfoUnderContract(input string, maxLines int, contract bool) (string, string, bool) {
	result := summarizeCargoBuildResult(input, maxLines, contract)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted " + strconv.Itoa(result.OmittedCount) + " additional lines")
}

// cargoDiagnosticBlock groups one rustc/clippy diagnostic: its
// `error[ENNNN]`/`warning:` header, primary span, offending source line,
// hints, and the lint name extracted from `#[warn(...)]` notes or lint-doc
// URLs.
type cargoDiagnosticBlock struct {
	header string
	lint   string
	spans  []string
	source string
	hints  []string
}

func summarizeCargoBuildResult(input string, maxLines int, contract bool) rustSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return rustSummaryResult{Text: "ok"}
	}

	collector, summaries, loose := collectCargoBuild(lines)
	if len(collector.blocks) == 0 && len(loose) == 0 {
		return fallbackCargoBuildSummary(input, summaries, collector.progress, maxLines)
	}
	return renderCargoBuildBlocks(collector.blocks, loose, summaries, len(collector.progress), maxLines, cargoBuildAllowance(input, maxLines, contract))
}

// collectCargoBuild classifies the output lines and returns the collector
// with its deduplicated, prioritized summaries and loose details.
func collectCargoBuild(lines []string) (*cargoBuildCollector, []string, []string) {
	collector := &cargoBuildCollector{}
	for _, line := range lines {
		collector.ingest(strings.TrimSpace(line))
	}
	summaries := prioritizeCargoSummaries(uniqueStrings(shared.FoldConsecutiveLines(collector.summaries)))
	loose := uniqueStrings(shared.FoldConsecutiveLines(collector.loose))
	return collector, summaries, loose
}

// cargoBuildAllowance predicts the compression-contract token allowance for
// an armed contract and disables the self-cap otherwise.
func cargoBuildAllowance(input string, maxLines int, contract bool) int {
	if !contract {
		return 0
	}
	return shared.PredictedTokenAllowance(input, maxLines)
}

func fallbackCargoBuildSummary(input string, summaries, progress []string, maxLines int) rustSummaryResult {
	combined := append([]string{}, summaries...)
	combined = append(combined, progress...)
	if len(combined) > 0 {
		return summarizeRustLines(combined, maxLines)
	}
	return rustSummaryResult{Text: shared.CompactLines(input, maxLines)}
}

// cargoBuildCollector groups output lines into diagnostic blocks, compile
// summaries, progress noise, and loose interesting details.
type cargoBuildCollector struct {
	blocks    []cargoDiagnosticBlock
	summaries []string
	progress  []string
	loose     []string
	inBlock   bool
}

func (c *cargoBuildCollector) ingest(trimmed string) {
	switch {
	case isCargoBuildSummaryLine(trimmed):
		c.summaries = append(c.summaries, clip(trimmed, 160))
		c.inBlock = false
	case strings.HasPrefix(trimmed, "Compiling "), strings.HasPrefix(trimmed, "Checking "), strings.HasPrefix(trimmed, "Downloaded "):
		c.progress = append(c.progress, clip(trimmed, 160))
		c.inBlock = false
	case isCargoDiagnosticHeader(trimmed):
		c.blocks = append(c.blocks, cargoDiagnosticBlock{header: clip(trimmed, 160)})
		c.inBlock = true
	case c.inBlock:
		collectCargoBlockLine(&c.blocks[len(c.blocks)-1], trimmed)
	case isCargoInterestingDetail(trimmed):
		c.loose = append(c.loose, clip(trimmed, 160))
	}
}

func collectCargoBlockLine(block *cargoDiagnosticBlock, trimmed string) {
	switch {
	case strings.HasPrefix(trimmed, "--> "):
		block.spans = append(block.spans, clip(trimmed, 160))
	case strings.HasPrefix(trimmed, "= help:"),
		strings.HasPrefix(trimmed, "= note:"),
		strings.HasPrefix(trimmed, "help:"),
		strings.HasPrefix(trimmed, "note:"):
		if lint := cargoLintName(trimmed); lint != "" && block.lint == "" {
			block.lint = lint
		}
		block.hints = append(block.hints, clip(trimmed, 160))
	case isCargoSourceLine(trimmed):
		if block.source == "" {
			block.source = clip(trimmed, 160)
		}
	}
}

// Selection tiers for cargo diagnostics: every diagnostic header (with its
// lint slug annotation) is irreducible, the leading compile summaries close
// the render, then spans, offending source lines, hints, and loose details
// fill what remains.
const (
	cargoTierHeader = iota
	cargoTierSummary
	cargoTierSpan
	cargoTierSource
	cargoTierHint
	cargoTierLoose
	cargoTierExtraHint
	cargoTierExtraSummary
)

// renderCargoBuildBlocks emits grouped diagnostics with tiered inclusion:
// every header always survives, then spans, offending source lines, and
// hints fill the remaining budget in that order, followed by the compile
// summary lines. The render self-caps to the predicted compression-contract
// allowance so every diagnostic header and lint slug reaches the display
// verbatim instead of gambling on the generic downstream token capper.
func renderCargoBuildBlocks(blocks []cargoDiagnosticBlock, loose, summaries []string, progressCount, maxLines, allowance int) rustSummaryResult {
	total := len(loose) + len(summaries) + progressCount + normalizeCargoBlocks(blocks)
	candidates := cargoBuildPriorityLines(blocks, loose, summaries)
	selected, omitted := shared.FitPriorityLines(candidates, maxLines, allowance)
	return rustSummaryResult{
		Text:         strings.Join(selected, "\n"),
		OmittedCount: omitted + total - len(candidates),
	}
}

// cargoBuildPriorityLines lays out each block's lines (header, spans,
// offending source, hints) in display order, followed by loose details and
// the compile summaries, tiered so every block's header survives before any
// block earns a second line.
func cargoBuildPriorityLines(blocks []cargoDiagnosticBlock, loose, summaries []string) []shared.PriorityLine {
	out := make([]shared.PriorityLine, 0, len(blocks)*3+len(loose)+len(summaries))
	for i := range blocks {
		out = append(out, cargoBlockPriorityLines(blocks[i])...)
	}
	for _, line := range loose {
		out = append(out, shared.PriorityLine{Text: line, Tier: cargoTierLoose})
	}
	for i, line := range summaries {
		tier := cargoTierExtraSummary
		if i < 2 {
			tier = cargoTierSummary
		}
		out = append(out, shared.PriorityLine{Text: line, Tier: tier})
	}
	return out
}

// cargoBlockPriorityLines lays out one diagnostic block in display order:
// header, spans, offending source line, then hints (first hint ahead of the
// per-block extras).
func cargoBlockPriorityLines(block cargoDiagnosticBlock) []shared.PriorityLine {
	out := []shared.PriorityLine{{Text: block.header, Tier: cargoTierHeader}}
	for _, span := range block.spans {
		out = append(out, shared.PriorityLine{Text: span, Tier: cargoTierSpan})
	}
	if block.source != "" {
		out = append(out, shared.PriorityLine{Text: block.source, Tier: cargoTierSource})
	}
	for j, hint := range block.hints {
		tier := cargoTierExtraHint
		if j == 0 {
			tier = cargoTierHint
		}
		out = append(out, shared.PriorityLine{Text: hint, Tier: tier})
	}
	return out
}

// normalizeCargoBlocks annotates lint names, prunes redundant hints, and
// returns the total candidate-line count across all blocks.
func normalizeCargoBlocks(blocks []cargoDiagnosticBlock) int {
	total := 0
	for i := range blocks {
		if blocks[i].lint != "" && !strings.Contains(blocks[i].header, blocks[i].lint) {
			blocks[i].header = clip(blocks[i].header+" ["+blocks[i].lint+"]", 160)
		}
		blocks[i].hints = pruneCargoLintDocHints(uniqueStrings(shared.FoldConsecutiveLines(blocks[i].hints)), blocks[i].lint)
		blocks[i].spans = shared.SelectUniqueAnchoredLines(blocks[i].spans, 2)
		total += 1 + len(blocks[i].spans) + len(blocks[i].hints)
		if blocks[i].source != "" {
			total++
		}
	}
	return total
}

func isCargoBuildSummaryLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "error: could not compile"),
		strings.HasPrefix(line, "error: aborting due to"),
		strings.HasPrefix(line, "Some errors have detailed explanations"),
		strings.HasPrefix(line, "For more information about"),
		strings.HasPrefix(line, "Finished "):
		return true
	case strings.HasPrefix(line, "warning:") && strings.Contains(line, "generated") && strings.Contains(line, "warning"):
		return true
	default:
		return false
	}
}

func prioritizeCargoSummaries(lines []string) []string {
	prioritized := []string{}
	for _, prefix := range []string{
		"error: could not compile",
		"error: aborting due to",
		"Some errors have detailed explanations",
		"warning:",
		"Finished ",
		"For more information about",
	} {
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				prioritized = append(prioritized, line)
			}
		}
	}
	if len(prioritized) == 0 {
		return lines
	}
	return uniqueStrings(prioritized)
}

// isCargoSourceLine matches rendered source lines like
// "142 |     let total: u64 = ...", which carry the flagged identifier.
func isCargoSourceLine(line string) bool {
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 {
		return false
	}
	rest := strings.TrimLeft(line[digits:], " ")
	return strings.HasPrefix(rest, "|") && strings.TrimSpace(strings.TrimPrefix(rest, "|")) != ""
}

// cargoLintName extracts a lint identifier from `#[warn(clippy::name)]`
// notes or clippy documentation URLs.
func cargoLintName(line string) string {
	for _, marker := range []string{"#[warn(", "#[deny(", "#[allow("} {
		if idx := strings.Index(line, marker); idx >= 0 {
			rest := line[idx+len(marker):]
			if end := strings.Index(rest, ")"); end > 0 {
				return rest[:end]
			}
		}
	}
	if idx := strings.Index(line, "rust-clippy/master/index.html#"); idx >= 0 {
		rest := line[idx+len("rust-clippy/master/index.html#"):]
		if fields := strings.Fields(rest); len(fields) > 0 {
			if slug := strings.TrimRight(fields[0], "`.,)"); slug != "" {
				return "clippy::" + slug
			}
		}
	}
	return ""
}

// pruneCargoLintDocHints drops hints that only restate an already-captured
// lint name (doc URLs and `#[warn(...)]` notes) so the budget goes to
// actionable suggestions instead.
func pruneCargoLintDocHints(hints []string, lint string) []string {
	if lint == "" {
		return hints
	}
	out := []string{}
	for _, hint := range hints {
		if strings.Contains(hint, "rust-clippy/master/index.html") || strings.Contains(hint, "#[warn(") || strings.Contains(hint, "#[deny(") {
			continue
		}
		out = append(out, hint)
	}
	return out
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func isCargoDiagnosticHeader(line string) bool {
	switch {
	case strings.HasPrefix(line, "error["),
		strings.HasPrefix(line, "warning:"),
		strings.HasPrefix(line, "error:"),
		strings.HasPrefix(line, "clippy::"):
		return true
	default:
		return false
	}
}

func isCargoFailureName(line string) bool {
	if line == "" {
		return false
	}
	if strings.Contains(line, "FAILED") || strings.Contains(line, "error:") {
		return false
	}
	return !strings.Contains(line, " ")
}

func isCargoInterestingDetail(line string) bool {
	switch {
	case line == "":
		return false
	case strings.HasPrefix(line, "--> "),
		strings.HasPrefix(line, "= help:"),
		strings.HasPrefix(line, "= note:"),
		strings.HasPrefix(line, "help:"),
		strings.HasPrefix(line, "note:"),
		strings.HasPrefix(line, "thread '"),
		strings.Contains(line, " panicked at "),
		strings.Contains(line, "Assertion"),
		strings.Contains(line, "assertion `"),
		strings.Contains(line, ".rs:"),
		strings.Contains(line, "unresolved import"),
		strings.Contains(line, "mismatched types"),
		strings.Contains(line, "unused "),
		strings.Contains(line, "cannot find "),
		strings.Contains(line, "failed to "),
		strings.Contains(line, "expected "),
		strings.Contains(line, "found "):
		return true
	default:
		return false
	}
}

func StripANSI(input string) string {
	return shared.StripANSI(input)
}

func clip(input string, max int) string {
	return shared.Clip(input, max)
}

func uniqueStrings(values []string) []string {
	return shared.UniqueStrings(values)
}

func nonEmptyLines(input string) []string {
	return shared.NonEmptyLines(input)
}

func joinLimitedLines(lines []string, maxLines int) string {
	return shared.JoinLimitedLines(lines, maxLines)
}

func summarizeRustLines(lines []string, maxLines int) rustSummaryResult {
	result := rustSummaryResult{Text: shared.JoinLimitedLines(lines, maxLines)}
	if len(lines) > maxLines {
		result.OmittedCount = len(lines) - maxLines
	}
	return result
}

func isDividerLine(line string) bool {
	return strings.Trim(line, "=-_ ") == ""
}

func pruneCargoFailureNames(lines []string) []string {
	if len(lines) <= 1 {
		return lines
	}
	explicit := map[string]struct{}{}
	for _, line := range lines {
		if strings.HasPrefix(line, "test ") && strings.Contains(line, " ... FAILED") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "test "), " ... FAILED"))
			explicit[name] = struct{}{}
		}
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := explicit[line]; ok {
			continue
		}
		out = append(out, line)
	}
	return out
}
