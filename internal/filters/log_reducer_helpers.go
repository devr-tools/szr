package filters

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type logLineKind uint8

const (
	logLineKindDefault logLineKind = iota
	logLineKindBoilerplate
	logLineKindHint
	logLineKindAnchor
	logLineKindError
)

type logLineEntry struct {
	key       string
	line      string
	firstSeen int
	lastSeen  int
	count     int
	kind      logLineKind
}

func orderedReducedLogEntries(entries []logLineEntry, selected map[string]struct{}) []logLineEntry {
	ordered := make([]logLineEntry, 0, len(selected))
	for _, entry := range entries {
		if _, ok := selected[entry.key]; ok {
			ordered = append(ordered, entry)
		}
	}
	return ordered
}

//nolint:maintidx // Entry collection intentionally combines dedupe, ordering, and signal promotion in one pass.
func collectLogLineEntries(lines []string) ([]logLineEntry, int) {
	indexByKey := map[string]int{}
	entries := make([]logLineEntry, 0, len(lines))
	total := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		total++

		key := reduceLogLineKey(line)
		if key == "" {
			continue
		}
		kind := classifyLogLine(line)
		if idx, ok := indexByKey[key]; ok {
			entries[idx].count++
			entries[idx].lastSeen = total - 1
			entries[idx].kind = maxLogLineKind(entries[idx].kind, kind)
			continue
		}

		indexByKey[key] = len(entries)
		entries = append(entries, logLineEntry{
			key:       key,
			line:      line,
			firstSeen: total - 1,
			lastSeen:  total - 1,
			count:     1,
			kind:      kind,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].firstSeen != entries[j].firstSeen {
			return entries[i].firstSeen < entries[j].firstSeen
		}
		return entries[i].lastSeen < entries[j].lastSeen
	})
	return entries, total
}

func reduceLogLineKey(line string) string {
	line = similarLineKey(line)
	line = strings.Join(strings.Fields(line), " ")
	return strings.TrimSpace(line)
}

func classifyLogLine(line string) logLineKind {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case lower == "":
		return logLineKindBoilerplate
	case isErrorLogLine(lower):
		return logLineKindError
	case isAnchorLogLine(line, lower):
		return logLineKindAnchor
	case isHintLogLine(lower):
		return logLineKindHint
	case isBoilerplateLogLine(lower):
		return logLineKindBoilerplate
	default:
		return logLineKindDefault
	}
}

var errorLogLevelTokens = []string{
	"panic",
	"fatal",
	"error",
	"failed",
	"failure",
	"exception",
	"traceback",
	"assertionerror",
}

var hintLogLevelTokens = []string{"warning", "warn", "hint", "help", "note"}

var boilerplateLogLevelTokens = []string{
	"debug",
	"info",
	"trace",
	"progress",
	"downloading",
	"downloaded",
	"fetching",
	"installing",
	"installed",
	"resolving",
	"waiting",
	"retrying",
	"connected",
	"heartbeat",
}

func isErrorLogLine(lower string) bool {
	if strings.Contains(lower, "segmentation fault") {
		return true
	}
	return hasLeadingLogLevelToken(lower, errorLogLevelTokens)
}

func isAnchorLogLine(line, lower string) bool {
	if DiagnosticAnchor(line) != "" {
		return true
	}
	if strings.HasPrefix(lower, "at ") || strings.HasPrefix(lower, "#") {
		return true
	}
	return strings.Contains(lower, "caused by:")
}

func isHintLogLine(lower string) bool {
	return hasLeadingLogLevelToken(lower, hintLogLevelTokens)
}

func isBoilerplateLogLine(lower string) bool {
	if strings.Contains(lower, "listening on") || strings.Contains(lower, "watching for changes") {
		return true
	}
	return hasLeadingLogLevelToken(lower, boilerplateLogLevelTokens)
}

// hasLeadingLogLevelToken reports whether one of the first few
// whitespace/bracket-delimited fields of the (lowercased) line matches a level
// marker. Token matching avoids substring misfires such as "information"
// matching "info" or "no errors" matching "error".
func hasLeadingLogLevelToken(lower string, markers []string) bool {
	const maxLogLevelTokenFields = 3
	fields := 0
	i := 0
	for i < len(lower) && fields < maxLogLevelTokenFields {
		raw, next, ok := nextLogToken(lower, i)
		i = next
		if !ok {
			break
		}
		token := strings.Trim(raw, logTokenPunctuationCutset)
		if token == "" {
			continue
		}
		fields++
		if matchesLogLevelMarker(token, markers) {
			return true
		}
	}
	return false
}

// nextLogToken scans the next delimiter-separated field of lower starting at
// pos, returning the field, the position after it, and ok=false when no field
// remains.
func nextLogToken(lower string, pos int) (token string, next int, ok bool) {
	i := pos
	for i < len(lower) && isLogTokenDelimiter(lower[i]) {
		i++
	}
	start := i
	for i < len(lower) && !isLogTokenDelimiter(lower[i]) {
		i++
	}
	if start == i {
		return "", i, false
	}
	return lower[start:i], i, true
}

// matchesLogLevelMarker reports whether token equals a marker ("error",
// "[warn]", "info:") or ends with one ("typeerror:", "assertionerror").
func matchesLogLevelMarker(token string, markers []string) bool {
	for _, marker := range markers {
		if token == marker || strings.HasSuffix(token, marker) {
			return true
		}
	}
	return false
}

const logTokenPunctuationCutset = ":.,;!?\"'`*-"

func isLogTokenDelimiter(c byte) bool {
	switch c {
	case ' ', '\t', '[', ']', '(', ')', '{', '}', '<', '>', '|', '=':
		return true
	}
	return false
}

func maxLogLineKind(left, right logLineKind) logLineKind {
	if right > left {
		return right
	}
	return left
}

// Canonical severities used by the log-summary aggregate.
const (
	logSeverityFatal = "FATAL"
	logSeverityError = "ERROR"
	logSeverityWarn  = "WARN"
	logSeverityInfo  = "INFO"
	logSeverityDebug = "DEBUG"
	logSeverityTrace = "TRACE"
	logSeverityOther = "other"
)

var logSeverityAliases = map[string]string{
	"fatal":    logSeverityFatal,
	"panic":    logSeverityFatal,
	"critical": logSeverityFatal,
	"crit":     logSeverityFatal,
	"error":    logSeverityError,
	"err":      logSeverityError,
	"warn":     logSeverityWarn,
	"warning":  logSeverityWarn,
	"info":     logSeverityInfo,
	"debug":    logSeverityDebug,
	"trace":    logSeverityTrace,
}

// logSeverityRenderOrder lists non-error severities in the order their
// distinct messages are rendered after the always-kept ERROR/FATAL block.
var logSeverityRenderOrder = []string{logSeverityWarn, logSeverityInfo, logSeverityDebug, logSeverityTrace, logSeverityOther}

func isErrorLogSeverity(severity string) bool {
	return severity == logSeverityError || severity == logSeverityFatal
}

// logSeverityAlias maps a raw field ("ERROR", "[warn]", "level=info") to its
// canonical severity.
func logSeverityAlias(field string) (string, bool) {
	token := strings.ToLower(strings.Trim(field, "[]()<>:,;|*"))
	if idx := strings.LastIndexByte(token, '='); idx >= 0 {
		token = token[idx+1:]
	}
	severity, ok := logSeverityAliases[token]
	return severity, ok
}

// detectLogSeverity finds a severity token in the first few message fields.
func detectLogSeverity(fields []string) string {
	limit := minInt(len(fields), 3)
	for i := 0; i < limit; i++ {
		if severity, ok := logSeverityAlias(fields[i]); ok {
			return severity
		}
	}
	return logSeverityOther
}

const logTimestampProbeSuffix = "szrtsprobe"

// leadingLogTimestampFields reports how many leading fields form a timestamp.
// It reuses the declarative SimilarLineKey timestamp detector by probing
// whether the candidate fields get stripped from a synthetic line, and adds
// the common two-field "YYYY-MM-DD HH:MM:SS" form the detector skips.
func leadingLogTimestampFields(fields []string) int {
	for n := 1; n <= 3 && n < len(fields); n++ {
		if isLogTimestampFieldRun(fields[:n]) {
			return n
		}
	}
	if len(fields) >= 3 && looksLikeLogDateField(fields[0]) && isLogTimestampFieldRun(fields[1:2]) {
		return 2
	}
	return 0
}

func isLogTimestampFieldRun(fields []string) bool {
	probe := strings.Join(fields, " ") + " " + logTimestampProbeSuffix
	return similarLineKey(probe) == logTimestampProbeSuffix
}

// looksLikeLogDateField recognizes bare "YYYY-MM-DD" or "YYYY/MM/DD" fields.
func looksLikeLogDateField(field string) bool {
	if len(field) != 10 {
		return false
	}
	if field[4] != field[7] || (field[4] != '-' && field[4] != '/') {
		return false
	}
	for i, c := range []byte(field) {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// extractLogClock returns the first "HH:MM:SS" substring found in fields.
func extractLogClock(fields []string) string {
	for _, field := range fields {
		for i := 0; i+8 <= len(field); i++ {
			if isLogClockCore(field[i : i+8]) {
				return field[i : i+8]
			}
		}
	}
	return ""
}

func isLogClockCore(core string) bool {
	for i := 0; i < 8; i++ {
		if i == 2 || i == 5 {
			if core[i] != ':' {
				return false
			}
			continue
		}
		if core[i] < '0' || core[i] > '9' {
			return false
		}
	}
	return true
}

type parsedLogLine struct {
	severity string
	clock    string
	display  string
}

func parseLogLine(line string) parsedLogLine {
	rest, clock := splitLogTimestamp(strings.Fields(line))
	return parsedLogLine{
		severity: detectLogSeverity(rest),
		clock:    clock,
		display:  strings.Join(rest, " "),
	}
}

// splitLogTimestamp strips a leading timestamp (optionally following a
// leading log-level token) and returns the remaining message fields plus the
// extracted HH:MM:SS clock, when present.
func splitLogTimestamp(fields []string) ([]string, string) {
	if n := leadingLogTimestampFields(fields); n > 0 {
		return fields[n:], extractLogClock(fields[:n])
	}
	if len(fields) >= 2 {
		if _, ok := logSeverityAlias(fields[0]); ok {
			if m := leadingLogTimestampFields(fields[1:]); m > 0 {
				rest := append([]string{fields[0]}, fields[1+m:]...)
				return rest, extractLogClock(fields[1 : 1+m])
			}
		}
	}
	return fields, ""
}

// isLogShapedLine reports whether a line starts with a timestamp or a
// log-level token — the per-line half of the log-content sniff.
func isLogShapedLine(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	if leadingLogTimestampFields(fields) > 0 {
		return true
	}
	limit := minInt(len(fields), 2)
	for i := 0; i < limit; i++ {
		if _, ok := logSeverityAlias(fields[i]); ok {
			return true
		}
	}
	return false
}

// formatLogCount renders an integer with thousands separators ("12,000").
func formatLogCount(n int) string {
	digits := strconv.Itoa(n)
	if n < 0 || len(digits) <= 3 {
		return digits
	}
	var out strings.Builder
	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}
	out.WriteString(digits[:lead])
	for i := lead; i < len(digits); i += 3 {
		out.WriteByte(',')
		out.WriteString(digits[i : i+3])
	}
	return out.String()
}

// Bounded-memory caps for the log-summary aggregate. Distinct messages
// beyond these caps are dropped from the dedup map but still counted in the
// severity histogram and the trailing "+N more distinct messages" note, so
// even a multi-gigabyte log with pathologically unique lines keeps memory
// bounded. Errors get their own cap so a flood of distinct low-severity
// messages can never evict error signal.
const (
	maxDistinctLogErrorEntries = 1024
	maxDistinctLogOtherEntries = 4096
	maxLogMessageLength        = 240
)

type logAggregateEntry struct {
	severity   string
	display    string
	count      int
	firstClock string
	lastClock  string
}

type logAggregate struct {
	totalLines      int
	severityCounts  map[string]int
	entries         map[string]*logAggregateEntry
	order           []*logAggregateEntry
	errorEntries    int
	otherEntries    int
	droppedDistinct int
}

func newLogAggregate() *logAggregate {
	return &logAggregate{
		severityCounts: map[string]int{},
		entries:        map[string]*logAggregateEntry{},
	}
}

func (a *logAggregate) ingest(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	a.totalLines++
	parsed := parseLogLine(line)
	a.severityCounts[parsed.severity]++
	a.recordEntry(parsed.severity+"\x00"+reduceLogLineKey(line), parsed)
}

func (a *logAggregate) recordEntry(key string, parsed parsedLogLine) {
	if entry, ok := a.entries[key]; ok {
		a.extendEntry(entry, parsed)
		return
	}
	if !a.admitEntry(parsed.severity) {
		a.droppedDistinct++
		return
	}
	entry := newLogAggregateEntry(parsed)
	a.entries[key] = entry
	a.order = append(a.order, entry)
}

func newLogAggregateEntry(parsed parsedLogLine) *logAggregateEntry {
	return &logAggregateEntry{
		severity:   parsed.severity,
		display:    Clip(parsed.display, maxLogMessageLength),
		count:      1,
		firstClock: parsed.clock,
		lastClock:  parsed.clock,
	}
}

func (a *logAggregate) extendEntry(entry *logAggregateEntry, parsed parsedLogLine) {
	entry.count++
	if parsed.clock == "" {
		return
	}
	if entry.firstClock == "" {
		entry.firstClock = parsed.clock
	}
	entry.lastClock = parsed.clock
}

// admitEntry enforces the bounded-memory caps on distinct messages.
func (a *logAggregate) admitEntry(severity string) bool {
	if isErrorLogSeverity(severity) {
		if a.errorEntries >= maxDistinctLogErrorEntries {
			return false
		}
		a.errorEntries++
		return true
	}
	if a.otherEntries >= maxDistinctLogOtherEntries {
		return false
	}
	a.otherEntries++
	return true
}

func (a *logAggregate) distinctTotal() int {
	return len(a.order) + a.droppedDistinct
}

// render produces the log summary: a severity histogram, every distinct
// ERROR/FATAL message (exempt from the line budget), then distinct WARN,
// INFO, DEBUG, TRACE, and other messages by count until the budget is spent.
func (a *logAggregate) render(maxLines int) []string {
	if a.totalLines == 0 {
		return nil
	}
	maxLines = normalizeReducedLogLimit(maxLines)
	lines := []string{a.histogramLine()}
	rendered := 0
	for _, entry := range a.errorLogEntries() {
		lines = append(lines, formatLogAggregateEntry(entry))
		rendered++
	}
	budget := maxLines - len(lines)
	for _, severity := range logSeverityRenderOrder {
		appended := a.appendTopEntries(&lines, severity, budget)
		budget -= appended
		rendered += appended
	}
	if omitted := a.distinctTotal() - rendered; omitted > 0 {
		lines = append(lines, fmt.Sprintf("... +%d more distinct messages", omitted))
	}
	return lines
}

type logSeverityCount struct {
	name  string
	count int
}

func (a *logAggregate) histogramLine() string {
	counts := sortedLogSeverityCounts(a.severityCounts)
	parts := make([]string, 0, len(counts))
	for _, sc := range counts {
		parts = append(parts, formatLogCount(sc.count)+" "+sc.name)
	}
	return fmt.Sprintf("%s lines: %s", formatLogCount(a.totalLines), strings.Join(parts, ", "))
}

func sortedLogSeverityCounts(severityCounts map[string]int) []logSeverityCount {
	counts := make([]logSeverityCount, 0, len(severityCounts))
	for name, count := range severityCounts {
		counts = append(counts, logSeverityCount{name: name, count: count})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].count != counts[j].count {
			return counts[i].count > counts[j].count
		}
		return counts[i].name < counts[j].name
	})
	return counts
}

// errorLogEntries returns every distinct ERROR/FATAL entry in first-seen
// order. These are never dropped, regardless of the output budget.
func (a *logAggregate) errorLogEntries() []*logAggregateEntry {
	out := []*logAggregateEntry{}
	for _, entry := range a.order {
		if isErrorLogSeverity(entry.severity) {
			out = append(out, entry)
		}
	}
	return out
}

func (a *logAggregate) appendTopEntries(lines *[]string, severity string, budget int) int {
	if budget <= 0 {
		return 0
	}
	group := []*logAggregateEntry{}
	for _, entry := range a.order {
		if entry.severity == severity {
			group = append(group, entry)
		}
	}
	sort.SliceStable(group, func(i, j int) bool { return group[i].count > group[j].count })
	appended := 0
	for _, entry := range group {
		if appended >= budget {
			break
		}
		*lines = append(*lines, formatLogAggregateEntry(entry))
		appended++
	}
	return appended
}

func formatLogAggregateEntry(entry *logAggregateEntry) string {
	parts := []string{}
	if entry.count > 1 {
		parts = append(parts, fmt.Sprintf("x%d", entry.count))
	}
	if clockRange := formatLogClockRange(entry); clockRange != "" {
		parts = append(parts, clockRange)
	}
	if len(parts) == 0 {
		return entry.display
	}
	return fmt.Sprintf("%s (%s)", entry.display, strings.Join(parts, ", "))
}

func formatLogClockRange(entry *logAggregateEntry) string {
	if entry.firstClock == "" {
		return ""
	}
	if entry.lastClock == "" || entry.lastClock == entry.firstClock {
		return entry.firstClock
	}
	return entry.firstClock + "–" + entry.lastClock
}
