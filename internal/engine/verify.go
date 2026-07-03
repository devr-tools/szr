package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/filters"
)

// The retention verifier makes szr's core design invariant mechanical: a
// render must never be less informative than the tokens it spends. After the
// full render pipeline finishes, the verifier extracts critical facts from
// the raw signal (failure lines, file:line anchors, diagnostic codes, test
// identifiers, summary counts), checks that each fact's identifying needle
// survived in the render, and reports the dropped facts so the repair step
// can append them. Whole-line survival is never required — a summarized form
// counts as retained as long as the identifying tokens survive.

const (
	// retentionMaxFacts bounds the single extraction pass. Past this many
	// critical facts the run is a diagnostic firehose and additional needles
	// stop adding verification signal.
	retentionMaxFacts = 64
	// retentionMaxNeedle keeps degenerate tokens (minified lines, embedded
	// blobs) from becoming needles no reasonable summary could retain.
	retentionMaxNeedle = 120
	// retentionMaxRepairLineRunes caps a single repaired line so one huge raw
	// line cannot dominate the repair section.
	retentionMaxRepairLineRunes = 200
)

type retentionFactKind int

const (
	retentionFactDetail retentionFactKind = iota
	retentionFactCount
)

type retentionFact struct {
	kind   retentionFactKind
	needle string
	line   string
}

// RetentionReport is the outcome of verifying one render against the raw
// signal it summarizes.
type RetentionReport struct {
	// Checked reports that verification ran over a complete raw source.
	Checked bool
	// MissingLines holds the deduped critical raw lines whose identifying
	// needles did not survive in the render, after count leniency. These are
	// the lines the repair step appends.
	MissingLines []string
	// MissingNeedles holds the identifying tokens that were dropped, aligned
	// with the facts behind MissingLines (multiple needles may share a line).
	MissingNeedles []string
	// MissingCounts counts summary-count facts that were not retained,
	// including ones the success-exit leniency chose not to queue for repair.
	MissingCounts int
}

// VerifyRetention extracts critical facts from rawSource and reports which of
// them rendered dropped. Missing summary counts are lenient: they are
// recorded but only queued for repair on a failure exit whose render retained
// no failure detail at all — a render that already carries the failing
// identifiers has spent its tokens better than bookkeeping numbers, and
// passing runs must never bloat over them.
func VerifyRetention(rawSource string, rendered string, failureExit bool) RetentionReport {
	report := RetentionReport{Checked: true}
	facts := extractRetentionFacts(rawSource)
	if len(facts) == 0 {
		return report
	}
	lowerRendered := strings.ToLower(rendered)
	retained, detailRetained := retainedRetentionFacts(facts, rendered, lowerRendered)
	collectMissingRetentionFacts(&report, facts, retained, failureExit && !detailRetained)
	return report
}

func collectMissingRetentionFacts(report *RetentionReport, facts []retentionFact, retained []bool, repairCounts bool) {
	seen := map[string]struct{}{}
	for i, fact := range facts {
		if retained[i] {
			continue
		}
		if fact.kind == retentionFactCount {
			report.MissingCounts++
			if !repairCounts {
				continue
			}
		}
		appendMissingRetentionFact(report, fact, seen)
	}
}

func appendMissingRetentionFact(report *RetentionReport, fact retentionFact, seen map[string]struct{}) {
	report.MissingNeedles = append(report.MissingNeedles, fact.needle)
	line := collapseRetentionLine(fact.line)
	if _, dup := seen[line]; dup {
		return
	}
	seen[line] = struct{}{}
	report.MissingLines = append(report.MissingLines, line)
}

func retainedRetentionFacts(facts []retentionFact, rendered string, lowerRendered string) ([]bool, bool) {
	retained := make([]bool, len(facts))
	detailRetained := false
	for i, fact := range facts {
		retained[i] = retentionFactRetained(fact, rendered, lowerRendered)
		if retained[i] && fact.kind == retentionFactDetail {
			detailRetained = true
		}
	}
	return retained, detailRetained
}

func retentionFactRetained(fact retentionFact, rendered string, lowerRendered string) bool {
	if fact.kind == retentionFactCount {
		return retentionCountRetained(lowerRendered, fact.needle)
	}
	return strings.Contains(rendered, fact.needle)
}

// retentionCountRetained accepts the common summary spellings of a count so
// renders that restate "3 failed" as "failed: 3" or "failed=3" still count.
func retentionCountRetained(lowerRendered string, needle string) bool {
	if strings.Contains(lowerRendered, needle) {
		return true
	}
	space := strings.IndexByte(needle, ' ')
	if space < 0 {
		return false
	}
	number, noun := needle[:space], needle[space+1:]
	for _, variant := range []string{noun + ": " + number, noun + "=" + number, noun + " " + number} {
		if strings.Contains(lowerRendered, variant) {
			return true
		}
	}
	return false
}

// extractRetentionFacts is a single bounded pass over the raw lines. Fact
// needles are deduped as they are found so repeated diagnostics cost one
// verification check.
func extractRetentionFacts(raw string) []retentionFact {
	facts := make([]retentionFact, 0, 16)
	seen := map[string]struct{}{}
	add := func(fact retentionFact) {
		if fact.needle == "" || len(fact.needle) > retentionMaxNeedle {
			return
		}
		if _, dup := seen[fact.needle]; dup {
			return
		}
		seen[fact.needle] = struct{}{}
		facts = append(facts, fact)
	}
	for offset := 0; offset < len(raw) && len(facts) < retentionMaxFacts; {
		line, next := nextRetentionLine(raw, offset)
		offset = next
		lineRetentionFacts(line, add)
	}
	return facts
}

func nextRetentionLine(raw string, offset int) (string, int) {
	end := strings.IndexByte(raw[offset:], '\n')
	if end < 0 {
		return raw[offset:], len(raw)
	}
	return raw[offset : offset+end], offset + end + 1
}

func lineRetentionFacts(line string, add func(retentionFact)) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if strings.IndexByte(line, 0x1b) >= 0 {
		line = strings.TrimSpace(filters.StripANSI(line))
	}
	// Structured runner output embeds multi-line diagnostics in one physical
	// line with escaped newlines; scan the encoded lines individually so
	// anchors are not mangled by the escape sequences around them.
	if strings.Contains(line, "\\n") {
		for _, encoded := range strings.Split(line, "\\n") {
			singleLineRetentionFacts(strings.TrimSpace(encoded), add)
		}
		return
	}
	singleLineRetentionFacts(line, add)
}

func singleLineRetentionFacts(line string, add func(retentionFact)) {
	if line == "" {
		return
	}
	lower := strings.ToLower(line)
	addRetentionCountFacts(line, lower, add)
	if !isCriticalRetentionLine(line, lower) {
		return
	}
	if anchor := filters.DiagnosticAnchor(line); anchor != "" {
		add(retentionFact{needle: retentionAnchorNeedle(anchor), line: line})
	}
	addRetentionCodeFacts(line, add)
	addRetentionFailureIdentifiers(line, add)
}

// retentionCriticalKeywords lists the lowercase shapes that classify a line
// as carrying exit-relevant diagnostic signal, mirroring what the failure
// reducers prioritize so verification checks what the pipeline promises to
// keep.
var retentionCriticalKeywords = []string{
	"error:", "error[", ": error ", "panic:", "fatal:",
	"exception", "traceback", "assertionerror", "undefined reference",
}

func isCriticalRetentionLine(line string, lower string) bool {
	if strings.Contains(line, "FAIL") || strings.Contains(line, "ERROR") {
		return true
	}
	// Stack and diagnostic-pointer lines carry the file:line anchor for the
	// error reported just above them.
	if strings.HasPrefix(line, "at ") || strings.HasPrefix(line, "--> ") {
		return true
	}
	return hasCriticalRetentionKeyword(lower)
}

func hasCriticalRetentionKeyword(lower string) bool {
	if strings.HasPrefix(lower, "error ") {
		return true
	}
	for _, keyword := range retentionCriticalKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// addRetentionCountFacts extracts numeric summary counts ("3 failed",
// "24 passed", "1 error(s)") from result lines. The cheap keyword gate keeps
// the field split off ordinary lines.
func addRetentionCountFacts(line string, lower string, add func(retentionFact)) {
	if !strings.Contains(lower, "passed") && !strings.Contains(lower, "failed") && !strings.Contains(lower, "error") {
		return
	}
	fields := strings.Fields(lower)
	for i := 0; i+1 < len(fields); i++ {
		if !allRetentionDigits(fields[i]) {
			continue
		}
		noun := retentionCountNoun(strings.Trim(fields[i+1], ".,;:!()[]"))
		if noun == "" {
			continue
		}
		add(retentionFact{kind: retentionFactCount, needle: fields[i] + " " + noun, line: line})
	}
}

func retentionCountNoun(token string) string {
	switch {
	case token == "failed" || token == "passed":
		return token
	case strings.HasPrefix(token, "error"):
		return "error"
	default:
		return ""
	}
}

// retentionAnchorNeedle reduces a file:line anchor to its identifying core:
// the base file name plus the first line number. Renders that shorten
// directories (".../pkg/file.go:12") or drop column numbers still count as
// retaining the anchor.
func retentionAnchorNeedle(anchor string) string {
	if idx := strings.LastIndexAny(anchor, "/\\"); idx >= 0 {
		anchor = anchor[idx+1:]
	}
	colon := strings.IndexByte(anchor, ':')
	if colon < 0 {
		return anchor
	}
	end := colon + 1
	for end < len(anchor) && anchor[end] >= '0' && anchor[end] <= '9' {
		end++
	}
	if end == colon+1 {
		return anchor[:colon]
	}
	return anchor[:end]
}

// addRetentionCodeFacts extracts diagnostic-code-shaped tokens: one to three
// uppercase letters followed by three to five digits with non-alphanumeric
// boundaries (E0599, CS1002, TS2339).
func addRetentionCodeFacts(line string, add func(retentionFact)) {
	for i := 0; i < len(line); {
		start, end := nextRetentionCode(line, i)
		if start < 0 {
			return
		}
		add(retentionFact{needle: line[start:end], line: line})
		i = end
	}
}

func nextRetentionCode(line string, from int) (int, int) {
	i := from
	for i < len(line) {
		if !isRetentionUpper(line[i]) {
			i++
			continue
		}
		if i > 0 && isRetentionAlnum(line[i-1]) {
			i = skipRetentionAlnum(line, i)
			continue
		}
		lettersEnd := skipRetentionUpper(line, i)
		digitsEnd := skipRetentionDigits(line, lettersEnd)
		if retentionCodeShapeOK(line, i, lettersEnd, digitsEnd) {
			return i, digitsEnd
		}
		i = skipRetentionAlnum(line, digitsEnd)
	}
	return -1, -1
}

func retentionCodeShapeOK(line string, start, lettersEnd, digitsEnd int) bool {
	letters := lettersEnd - start
	digits := digitsEnd - lettersEnd
	if letters < 1 || letters > 3 || digits < 3 || digits > 5 {
		return false
	}
	return digitsEnd == len(line) || !isRetentionAlnum(line[digitsEnd])
}

// addRetentionFailureIdentifiers extracts the identifying token that follows
// an explicit failure marker, covering runner shapes like "--- FAIL:
// TestReplayWindow" and "FAILED tests/test_auth.py::test_login".
func addRetentionFailureIdentifiers(line string, add func(retentionFact)) {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		if !isRetentionFailureMarker(fields[i]) {
			continue
		}
		if needle := retentionIdentifierNeedle(fields[i+1]); needle != "" {
			add(retentionFact{needle: needle, line: line})
		}
	}
}

func isRetentionFailureMarker(field string) bool {
	switch strings.Trim(field, ":.") {
	case "FAIL", "FAILED", "FAILURE":
		return true
	default:
		return false
	}
}

// retentionIdentifierNeedle normalizes a marker-adjacent token to the part a
// summarized render must keep: quoting and bracket punctuation stripped, and
// leading directories dropped so shortened paths still count.
func retentionIdentifierNeedle(token string) string {
	token = strings.Trim(token, "\"'`()[]{}<>,;:!")
	if idx := strings.LastIndexAny(token, "/\\"); idx >= 0 {
		token = token[idx+1:]
	}
	if len(token) < 3 || allRetentionDigits(token) {
		return ""
	}
	return token
}

func collapseRetentionLine(line string) string {
	return clipRunes(strings.Join(strings.Fields(line), " "), retentionMaxRepairLineRunes)
}

func allRetentionDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isRetentionDigit(value[i]) {
			return false
		}
	}
	return true
}

func isRetentionUpper(ch byte) bool {
	return ch >= 'A' && ch <= 'Z'
}

func isRetentionDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isRetentionAlnum(ch byte) bool {
	return isRetentionUpper(ch) || isRetentionDigit(ch) || (ch >= 'a' && ch <= 'z')
}

func skipRetentionUpper(line string, from int) int {
	for from < len(line) && isRetentionUpper(line[from]) {
		from++
	}
	return from
}

func skipRetentionDigits(line string, from int) int {
	for from < len(line) && isRetentionDigit(line[from]) {
		from++
	}
	return from
}

func skipRetentionAlnum(line string, from int) int {
	for from < len(line) && isRetentionAlnum(line[from]) {
		from++
	}
	return from
}
