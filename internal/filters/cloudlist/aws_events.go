package cloudlist

import (
	"fmt"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeAWSLogEvents(input string, maxLines int) string {
	return summarizeAWSLogEventsResult(input, maxLines).Text
}

func AWSLogEventsRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return cloudListRecovery(summarizeAWSLogEventsResult(input, maxLines), "event groups")
}

// logTemplateGroup folds log events whose messages differ only in numbers or
// identifiers into one line: the first concrete message stands in for the
// group and the count carries the repetition.
type logTemplateGroup struct {
	example string
	count   int
	isError bool
}

func summarizeAWSLogEventsResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}
	var events []map[string]any
	if decoded := awsObject(input); decoded != nil {
		events = objectSlice(decoded["events"])
	}
	if len(events) == 0 {
		return cloudListSummaryResult{Text: shared.CompactLines(shared.StripANSI(input), maxLines)}
	}

	scan := groupLogEvents(events)
	selected, omitted := shared.FitPriorityLines(scan.candidates(), bodyLineLimit(maxLines), 0)
	return cloudListSummaryResult{
		Text:         strings.Join(append([]string{scan.header(len(events))}, selected...), "\n"),
		OmittedCount: omitted,
	}
}

// logEventScan accumulates the per-template groups plus the stream and error
// tallies for the header line.
type logEventScan struct {
	groups     map[string]*logTemplateGroup
	order      []string
	streams    map[string]struct{}
	errorCount int
}

func groupLogEvents(events []map[string]any) *logEventScan {
	scan := &logEventScan{groups: map[string]*logTemplateGroup{}, streams: map[string]struct{}{}}
	for _, event := range events {
		scan.ingest(event)
	}
	return scan
}

func (s *logEventScan) ingest(event map[string]any) {
	message := strings.TrimSpace(firstString(event, "message"))
	if message == "" {
		return
	}
	if stream := firstString(event, "logStreamName"); stream != "" {
		s.streams[stream] = struct{}{}
	}
	group := s.group(awsLogTemplate(message), message)
	group.count++
	if group.isError {
		s.errorCount++
	}
}

func (s *logEventScan) group(key, message string) *logTemplateGroup {
	if group, seen := s.groups[key]; seen {
		return group
	}
	group := &logTemplateGroup{example: message, isError: isAWSLogErrorMessage(message)}
	s.groups[key] = group
	s.order = append(s.order, key)
	return group
}

func (s *logEventScan) header(total int) string {
	header := fmt.Sprintf("log events: %d", total)
	if s.errorCount > 0 {
		header += fmt.Sprintf(" (errors=%d)", s.errorCount)
	}
	if len(s.streams) > 0 {
		header += fmt.Sprintf(" streams=%d", len(s.streams))
	}
	return header
}

// candidates renders one line per template group: error templates are
// irreducible and lead, the rest follow ordered by repetition count.
func (s *logEventScan) candidates() []shared.PriorityLine {
	sort.SliceStable(s.order, func(i, j int) bool {
		left, right := s.groups[s.order[i]], s.groups[s.order[j]]
		if left.isError != right.isError {
			return left.isError
		}
		return left.count > right.count
	})
	out := make([]shared.PriorityLine, 0, len(s.order))
	for _, key := range s.order {
		out = append(out, s.groups[key].priorityLine())
	}
	return out
}

func (g *logTemplateGroup) priorityLine() shared.PriorityLine {
	tier := 1
	if g.isError {
		tier = 0
	}
	line := shared.Clip(g.example, 160)
	if g.count > 1 {
		line = fmt.Sprintf("x%d %s", g.count, line)
	}
	return shared.PriorityLine{Text: line, Tier: tier}
}

// awsLogTemplate normalizes a log message into its repetition signature:
// digit runs and long hex identifiers collapse so retries of the same
// statement with different ids, offsets, or durations group together.
func awsLogTemplate(message string) string {
	fields := strings.Fields(message)
	for i, field := range fields {
		fields[i] = normalizeLogToken(field)
	}
	return strings.Join(fields, " ")
}

func normalizeLogToken(field string) string {
	if isHexLikeToken(field) {
		return "#"
	}
	out := make([]byte, 0, len(field))
	lastDigit := false
	for i := 0; i < len(field); i++ {
		ch := field[i]
		if ch >= '0' && ch <= '9' {
			if !lastDigit {
				out = append(out, 'N')
			}
			lastDigit = true
			continue
		}
		lastDigit = false
		out = append(out, ch)
	}
	return string(out)
}

func isHexLikeToken(field string) bool {
	cleaned := strings.Trim(field, "\"'()[]{},:")
	if len(cleaned) < 8 {
		return false
	}
	hasDigit := false
	for i := 0; i < len(cleaned); i++ {
		ch := cleaned[i]
		switch {
		case ch >= '0' && ch <= '9':
			hasDigit = true
		case ch >= 'a' && ch <= 'f', ch >= 'A' && ch <= 'F', ch == '-':
		default:
			return false
		}
	}
	return hasDigit
}

func isAWSLogErrorMessage(message string) bool {
	lower := strings.ToLower(message)
	for _, fragment := range []string{"error", "exception", "fatal", "panic", "fail", "timed out", "timeout", "throttl", "denied"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func SummarizeAWSStackEvents(input string, maxLines int) string {
	return summarizeAWSStackEventsResult(input, maxLines).Text
}

func AWSStackEventsRecoveryInfo(input string, maxLines int) (string, string, bool) {
	return cloudListRecovery(summarizeAWSStackEventsResult(input, maxLines), "stack events")
}

func summarizeAWSStackEventsResult(input string, maxLines int) cloudListSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}
	events := awsCollection(input, "StackEvents")
	if len(events) == 0 {
		return summarizeCloudListResult(input, maxLines)
	}

	candidates, failed := stackEventCandidates(events)
	header := fmt.Sprintf("stack events: %d", len(events))
	if failed > 0 {
		header += fmt.Sprintf(" (failed=%d)", failed)
	}
	selected, omitted := shared.FitPriorityLines(candidates, bodyLineLimit(maxLines), 0)
	return cloudListSummaryResult{
		Text:         strings.Join(append([]string{header}, selected...), "\n"),
		OmittedCount: omitted,
	}
}

// bodyLineLimit reserves one line for the summary header.
func bodyLineLimit(maxLines int) int {
	if maxLines <= 2 {
		return 1
	}
	return maxLines - 1
}

// stackEventCandidates keeps failed events first (they name the resource and
// the reason the stack operation broke) and folds repeated cancellation
// noise, then follows with the newest progress events in source order.
func stackEventCandidates(events []map[string]any) ([]shared.PriorityLine, int) {
	scan := &stackEventScan{seenReasons: map[string]int{}}
	for _, event := range events {
		scan.ingest(event)
	}
	return append(scan.failures, scan.progress...), scan.failed
}

type stackEventScan struct {
	failures    []shared.PriorityLine
	progress    []shared.PriorityLine
	seenReasons map[string]int
	failed      int
}

func (s *stackEventScan) ingest(event map[string]any) {
	status := firstString(event, "ResourceStatus")
	if !isStackFailureStatus(status) {
		s.progress = append(s.progress, shared.PriorityLine{Text: shared.Clip(status+" "+firstString(event, "LogicalResourceId"), 160), Tier: 1})
		return
	}
	s.failed++
	key := status + "|" + firstString(event, "ResourceStatusReason")
	if idx, dup := s.seenReasons[key]; dup {
		s.failures[idx].Text = bumpRepeatSuffix(s.failures[idx].Text)
		return
	}
	s.seenReasons[key] = len(s.failures)
	s.failures = append(s.failures, shared.PriorityLine{Text: shared.Clip(formatStackFailure(event, status), 200), Tier: 0})
}

func formatStackFailure(event map[string]any, status string) string {
	line := status + " " + firstString(event, "LogicalResourceId")
	if resourceType := firstString(event, "ResourceType"); resourceType != "" {
		line += " (" + resourceType + ")"
	}
	if reason := firstString(event, "ResourceStatusReason"); reason != "" {
		line += ": " + reason
	}
	return line
}

// bumpRepeatSuffix increments the trailing "(xN)" marker on a folded line.
func bumpRepeatSuffix(text string) string {
	open := strings.LastIndex(text, " (x")
	if open < 0 || !strings.HasSuffix(text, ")") {
		return text + " (x2)"
	}
	count := 0
	if _, err := fmt.Sscanf(text[open:], " (x%d)", &count); err != nil || count == 0 {
		return text + " (x2)"
	}
	return fmt.Sprintf("%s (x%d)", text[:open], count+1)
}

func isStackFailureStatus(status string) bool {
	return strings.Contains(status, "FAILED") || strings.Contains(status, "ROLLBACK")
}
