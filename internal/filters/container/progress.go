package container

import (
	"fmt"
	"regexp"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var dockerLayerLinePattern = regexp.MustCompile(`^[0-9a-f]{6,64}: `)

var composeResourceStatePattern = regexp.MustCompile(`^(Container|Network|Volume|Image)\s+(\S+)\s+(\S.*)$`)

type containerSummaryResult struct {
	Text         string
	OmittedCount int
}

func limitContainerLines(lines []string, maxLines int) containerSummaryResult {
	result := containerSummaryResult{Text: shared.JoinLimitedLines(lines, maxLines)}
	if len(lines) > maxLines {
		result.OmittedCount = len(lines) - maxLines
	}
	return result
}

func SummarizeDockerTransfer(input string, maxLines int) string {
	return summarizeDockerTransferResult(input, maxLines).Text
}

func DockerTransferRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDockerTransferResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type dockerTransferScan struct {
	layerStatus map[string]string
	pushSeen    bool
	errorLines  []string
	kept        []string
}

func newDockerTransferScan() *dockerTransferScan {
	return &dockerTransferScan{layerStatus: map[string]string{}}
}

func (s *dockerTransferScan) ingestLine(line string) {
	trimmed := strings.TrimSpace(line)
	if match := dockerLayerLinePattern.FindString(trimmed); match != "" {
		s.ingestLayerLine(trimmed, match)
		return
	}
	if strings.Contains(trimmed, "The push refers to") {
		s.pushSeen = true
	}
	lower := strings.ToLower(trimmed)
	switch {
	case isDockerTransferErrorText(lower):
		s.errorLines = append(s.errorLines, shared.Clip(trimmed, 160))
	case isDockerTransferKeptText(trimmed, lower):
		s.kept = append(s.kept, shared.Clip(trimmed, 160))
	}
}

func (s *dockerTransferScan) ingestLayerLine(trimmed, match string) {
	id := strings.TrimSuffix(match, ": ")
	status := strings.TrimSpace(strings.TrimPrefix(trimmed, match))
	s.layerStatus[id] = status
	lowerStatus := strings.ToLower(status)
	if strings.Contains(lowerStatus, "error") || strings.Contains(lowerStatus, "failed") {
		s.errorLines = append(s.errorLines, shared.Clip(trimmed, 160))
	}
}

func isDockerTransferErrorText(lower string) bool {
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "denied") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "manifest unknown")
}

func isDockerTransferKeptText(trimmed, lower string) bool {
	return strings.Contains(trimmed, "Pulling from ") ||
		strings.Contains(trimmed, "The push refers to") ||
		strings.HasPrefix(trimmed, "Digest:") ||
		strings.HasPrefix(trimmed, "Status:") ||
		strings.Contains(lower, "digest: sha256:")
}

func (s *dockerTransferScan) empty() bool {
	return len(s.layerStatus) == 0 && len(s.errorLines) == 0 && len(s.kept) == 0
}

func (s *dockerTransferScan) layerSummaryLine() string {
	existing := 0
	pushSeen := s.pushSeen
	for _, status := range s.layerStatus {
		lowerStatus := strings.ToLower(status)
		if strings.Contains(lowerStatus, "already exists") || strings.Contains(lowerStatus, "mounted from") {
			existing++
		}
		if strings.Contains(lowerStatus, "push") || strings.Contains(lowerStatus, "prepar") {
			pushSeen = true
		}
	}
	verb := "pulled"
	if pushSeen {
		verb = "pushed"
	}
	layerLine := fmt.Sprintf("%s %d layers", verb, len(s.layerStatus)-existing)
	if existing > 0 {
		layerLine += fmt.Sprintf(" (%d already existed)", existing)
	}
	return layerLine
}

func (s *dockerTransferScan) buildOutput() []string {
	out := []string{}
	if len(s.layerStatus) > 0 {
		out = append(out, s.layerSummaryLine())
	}
	out = append(out, s.errorLines...)
	out = append(out, s.kept...)
	return out
}

func summarizeDockerTransferResult(input string, maxLines int) containerSummaryResult {
	if maxLines <= 0 {
		maxLines = 8
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return containerSummaryResult{Text: "ok"}
	}

	scan := newDockerTransferScan()
	for _, line := range lines {
		scan.ingestLine(line)
	}

	scan.errorLines = shared.UniqueStrings(scan.errorLines)
	scan.kept = shared.UniqueStrings(scan.kept)
	if scan.empty() {
		return containerSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}
	return limitContainerLines(scan.buildOutput(), maxLines)
}

func SummarizeComposeActivity(input string, maxLines int) string {
	return summarizeComposeActivityResult(input, maxLines).Text
}

func ComposeActivityRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeComposeActivityResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional lines", result.OmittedCount))
}

type composeActivityScan struct {
	states           map[string]string
	order            []string
	startedEvents    int
	healthyEvents    int
	errorLines       []string
	attach           []string
	lastProgress     string
	inErrorBlock     bool
	errorBlockBudget int
}

// composeErrorBlockBudget bounds how many lines of BuildKit's dash-delimited
// failure blocks are retained.
const composeErrorBlockBudget = 8

func newComposeActivityScan() *composeActivityScan {
	return &composeActivityScan{states: map[string]string{}, errorBlockBudget: composeErrorBlockBudget}
}

func (s *composeActivityScan) ingestLine(line string) {
	trimmed := strings.TrimSpace(line)
	if s.ingestErrorBlockLine(trimmed) {
		return
	}
	if match := composeResourceStatePattern.FindStringSubmatch(trimmed); match != nil {
		s.ingestResourceState(match)
		return
	}
	if strings.HasPrefix(trimmed, "[+]") {
		s.lastProgress = trimmed
		return
	}
	if isComposeErrorText(strings.ToLower(trimmed)) {
		s.errorLines = append(s.errorLines, shared.Clip(trimmed, 160))
		return
	}
	if strings.HasPrefix(trimmed, "#") {
		// BuildKit progress lines; failures were already captured above.
		return
	}
	if source, message, ok := strings.Cut(line, " | "); ok {
		s.ingestAttachLine(source, message)
	}
}

// ingestErrorBlockLine keeps the lines inside BuildKit's dash-delimited
// failure blocks: that echo of the failed step's output is where the actual
// compiler or command error lives, and its detail lines rarely contain an
// "error" keyword of their own.
func (s *composeActivityScan) ingestErrorBlockLine(trimmed string) bool {
	if isDashDelimiterLine(trimmed) {
		s.inErrorBlock = !s.inErrorBlock
		return true
	}
	if !s.inErrorBlock {
		return false
	}
	if s.errorBlockBudget > 0 {
		s.errorLines = append(s.errorLines, shared.Clip(trimmed, 160))
		s.errorBlockBudget--
	}
	return true
}

func isDashDelimiterLine(trimmed string) bool {
	if len(trimmed) < 4 {
		return false
	}
	for _, r := range trimmed {
		if r != '-' {
			return false
		}
	}
	return true
}

func (s *composeActivityScan) ingestResourceState(match []string) {
	key := match[1] + " " + match[2]
	state := strings.TrimSpace(match[3])
	if _, seen := s.states[key]; !seen {
		s.order = append(s.order, key)
	}
	s.states[key] = state
	switch {
	case strings.HasPrefix(state, "Started"):
		s.startedEvents++
	case strings.HasPrefix(state, "Healthy"):
		s.healthyEvents++
	}
}

func (s *composeActivityScan) ingestAttachLine(source, message string) {
	if isInterestingLogLine(message) {
		s.attach = append(s.attach, shared.Clip(strings.TrimSpace(source)+": "+strings.TrimSpace(message), 160))
	}
}

func isComposeErrorText(lower string) bool {
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "exit code") ||
		strings.Contains(lower, "did not complete successfully")
}

func (s *composeActivityScan) finalize() {
	s.errorLines = shared.UniqueStrings(s.errorLines)
	s.attach = shared.UniqueStrings(s.attach)
}

func (s *composeActivityScan) empty() bool {
	return len(s.order) == 0 && len(s.errorLines) == 0 && len(s.attach) == 0
}

func (s *composeActivityScan) fallbackResult(clean string, maxLines int) containerSummaryResult {
	if s.lastProgress != "" {
		return limitContainerLines([]string{shared.Clip(s.lastProgress, 160)}, maxLines)
	}
	return containerSummaryResult{Text: shared.CompactLines(clean, maxLines)}
}

func (s *composeActivityScan) buildOutput() []string {
	out := []string{}
	if len(s.order) > 0 {
		out = append(out, fmt.Sprintf("services: started=%d healthy=%d", s.startedEvents, s.healthyEvents))
	}
	out = append(out, s.errorLines...)
	for _, key := range s.order {
		state := s.states[key]
		if composeIntermediateState(state) {
			continue
		}
		out = append(out, shared.Clip(key+" "+state, 160))
	}
	out = append(out, s.attach...)
	if len(out) == 0 && s.lastProgress != "" {
		out = append(out, shared.Clip(s.lastProgress, 160))
	}
	return out
}

func summarizeComposeActivityResult(input string, maxLines int) containerSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return containerSummaryResult{Text: "ok"}
	}

	scan := newComposeActivityScan()
	for _, line := range lines {
		scan.ingestLine(line)
	}

	scan.finalize()
	if scan.empty() {
		return scan.fallbackResult(clean, maxLines)
	}
	return limitContainerLines(scan.buildOutput(), maxLines)
}

func composeIntermediateState(state string) bool {
	first := state
	if idx := strings.IndexByte(state, ' '); idx > 0 {
		first = state[:idx]
	}
	switch first {
	case "Creating", "Starting", "Stopping", "Removing", "Recreating", "Restarting", "Waiting", "Building", "Pulling":
		return true
	default:
		return false
	}
}
