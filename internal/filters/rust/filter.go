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
	return summarizeCargoBuildResult(input, maxLines).Text
}

func CargoBuildRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeCargoBuildResult(input, maxLines)
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

func summarizeCargoBuildResult(input string, maxLines int) rustSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := nonEmptyLines(StripANSI(input))
	if len(lines) == 0 {
		return rustSummaryResult{Text: "ok"}
	}

	blocks := []cargoDiagnosticBlock{}
	summaries := []string{}
	progress := []string{}
	loose := []string{}
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case isCargoBuildSummaryLine(trimmed):
			summaries = append(summaries, clip(trimmed, 160))
			inBlock = false
		case strings.HasPrefix(trimmed, "Compiling "), strings.HasPrefix(trimmed, "Checking "), strings.HasPrefix(trimmed, "Downloaded "):
			progress = append(progress, clip(trimmed, 160))
			inBlock = false
		case isCargoDiagnosticHeader(trimmed):
			blocks = append(blocks, cargoDiagnosticBlock{header: clip(trimmed, 160)})
			inBlock = true
		case inBlock:
			collectCargoBlockLine(&blocks[len(blocks)-1], trimmed)
		case isCargoInterestingDetail(trimmed):
			loose = append(loose, clip(trimmed, 160))
		}
	}

	summaries = prioritizeCargoSummaries(uniqueStrings(shared.FoldConsecutiveLines(summaries)))
	loose = uniqueStrings(shared.FoldConsecutiveLines(loose))

	if len(blocks) == 0 && len(loose) == 0 {
		combined := append([]string{}, summaries...)
		combined = append(combined, progress...)
		if len(combined) > 0 {
			return summarizeRustLines(combined, maxLines)
		}
		return rustSummaryResult{Text: shared.CompactLines(input, maxLines)}
	}

	return renderCargoBuildBlocks(blocks, loose, summaries, len(progress), maxLines)
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

// renderCargoBuildBlocks emits grouped diagnostics with tiered inclusion:
// every header always survives, then spans, offending source lines, and
// hints fill the remaining budget in that order, followed by the compile
// summary lines.
func renderCargoBuildBlocks(blocks []cargoDiagnosticBlock, loose, summaries []string, progressCount, maxLines int) rustSummaryResult {
	total := len(loose) + len(summaries) + progressCount + normalizeCargoBlocks(blocks)
	budget := cargoBlockBudget(len(blocks), len(summaries), maxLines)

	// Every line selected so far counts against the block budget; loose
	// details may fill what remains of it before summaries take the tail.
	include, _ := selectCargoBlockLines(blocks, budget)
	out := []string{}
	for i := range blocks {
		out = append(out, include[i]...)
	}
	out = appendWithinLimit(out, loose, minInt(budget, maxLines))
	out = appendWithinLimit(out, summaries, maxLines)

	result := rustSummaryResult{Text: joinLimitedLines(out, maxLines)}
	if total > len(out) {
		result.OmittedCount = total - len(out)
	}
	return result
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

func cargoBlockBudget(blockCount, summaryCount, maxLines int) int {
	reserve := minInt(summaryCount, 2)
	budget := maxLines - reserve
	if budget < blockCount {
		budget = blockCount
	}
	return budget
}

// selectCargoBlockLines picks grouped lines per block in priority tiers so
// every header survives before any block gets a second line.
func selectCargoBlockLines(blocks []cargoDiagnosticBlock, budget int) ([][]string, int) {
	include := make([][]string, len(blocks))
	used := 0
	for i := range blocks {
		include[i] = []string{blocks[i].header}
		used++
	}
	for _, pick := range cargoBlockTiers(blocks) {
		for i := range blocks {
			if used >= budget {
				return include, used
			}
			if lines := pick(i); len(lines) > 0 {
				include[i] = append(include[i], lines[0])
				used++
			}
		}
	}
	return appendCargoExtraHints(blocks, include, used, budget)
}

// cargoBlockTiers orders each block's candidate lines: primary span first,
// then the offending source line, then the first hint.
func cargoBlockTiers(blocks []cargoDiagnosticBlock) []func(i int) []string {
	return []func(i int) []string{
		func(i int) []string { return blocks[i].spans },
		func(i int) []string {
			if blocks[i].source == "" {
				return nil
			}
			return []string{blocks[i].source}
		},
		func(i int) []string { return blocks[i].hints },
	}
}

// appendCargoExtraHints adds each block's remaining hints once every block
// already received its tiered lines.
func appendCargoExtraHints(blocks []cargoDiagnosticBlock, include [][]string, used, budget int) ([][]string, int) {
	for i := range blocks {
		for _, hint := range blocks[i].hints[minInt(1, len(blocks[i].hints)):] {
			if used >= budget {
				return include, used
			}
			include[i] = append(include[i], hint)
			used++
		}
	}
	return include, used
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
