package build

import (
	"fmt"
	"regexp"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

var (
	buildKitStepPattern   = regexp.MustCompile(`^#\d+ `)
	buildKitHeaderPattern = regexp.MustCompile(`^(#\d+) (\[.+)$`)
	buildKitErrorPattern  = regexp.MustCompile(`^(#\d+) (ERROR|CANCELED)\b`)
)

// isBuildKitOutput reports whether the output is a BuildKit progress stream
// (docker build / docker buildx build). These need dedicated handling: the
// step that failed carries its compiler or command error on plain detail
// lines inside dash-delimited blocks, which the generic build-line rules
// would drop.
func isBuildKitOutput(clean string) bool {
	matches := 0
	for _, line := range shared.NonEmptyLines(clean) {
		if buildKitStepPattern.MatchString(strings.TrimSpace(line)) {
			matches++
			if matches >= 3 {
				return true
			}
		}
	}
	return false
}

// buildKitScan collects the failure payload of a BuildKit stream: the failed
// step's header and ERROR line, every line of the dash-delimited error and
// Dockerfile-pointer blocks (that is where the actual compiler error lives),
// and the final ERROR:/Dockerfile: lines outside any block.
type buildKitScan struct {
	stepHeaders map[string]string
	stepOrder   []string
	failedSteps []string
	errorLines  []string
	blockLines  []string
	tailLines   []string
	inBlock     bool
	blockBudget int
}

const buildKitBlockLineBudget = 12

func newBuildKitScan() *buildKitScan {
	return &buildKitScan{stepHeaders: map[string]string{}, blockBudget: buildKitBlockLineBudget}
}

func (s *buildKitScan) ingestLine(trimmed string) {
	if isBuildKitBlockDelimiter(trimmed) {
		s.inBlock = !s.inBlock
		return
	}
	if s.inBlock {
		s.ingestBlockLine(trimmed)
		return
	}
	if s.ingestStepLine(trimmed) {
		return
	}
	if isBuildKitTailLine(trimmed) {
		s.tailLines = append(s.tailLines, shared.Clip(trimmed, 200))
	}
}

func (s *buildKitScan) ingestStepLine(trimmed string) bool {
	if match := buildKitErrorPattern.FindStringSubmatch(trimmed); match != nil {
		s.recordFailedStep(match[1])
		s.errorLines = append(s.errorLines, shared.Clip(trimmed, 200))
		return true
	}
	if match := buildKitHeaderPattern.FindStringSubmatch(trimmed); match != nil {
		s.recordStepHeader(match[1], trimmed)
		return true
	}
	return false
}

func isBuildKitTailLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "ERROR:") ||
		strings.HasPrefix(trimmed, "Dockerfile:") ||
		strings.HasPrefix(trimmed, "WARNING:")
}

func (s *buildKitScan) ingestBlockLine(trimmed string) {
	if s.blockBudget <= 0 {
		return
	}
	s.blockLines = append(s.blockLines, shared.Clip(trimmed, 200))
	s.blockBudget--
}

func (s *buildKitScan) recordFailedStep(id string) {
	for _, existing := range s.failedSteps {
		if existing == id {
			return
		}
	}
	s.failedSteps = append(s.failedSteps, id)
}

func (s *buildKitScan) recordStepHeader(id, line string) {
	if _, seen := s.stepHeaders[id]; !seen {
		s.stepOrder = append(s.stepOrder, id)
	}
	s.stepHeaders[id] = shared.Clip(line, 160)
}

// isBuildKitBlockDelimiter matches the dash-only separator lines BuildKit
// prints around the failed step's output echo and the Dockerfile pointer.
func isBuildKitBlockDelimiter(trimmed string) bool {
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

func (s *buildKitScan) failed() bool {
	return len(s.failedSteps) > 0 || len(s.blockLines) > 0 || len(s.tailLines) > 0
}

func (s *buildKitScan) failureOutput() []string {
	out := []string{}
	for _, id := range s.failedSteps {
		if header, ok := s.stepHeaders[id]; ok {
			out = append(out, header)
		}
	}
	out = append(out, s.errorLines...)
	out = append(out, s.blockLines...)
	out = append(out, s.tailLines...)
	return shared.UniqueStrings(out)
}

func (s *buildKitScan) successOutput(clean string, maxLines int) []string {
	out := []string{fmt.Sprintf("build ok: %d steps", len(s.stepOrder))}
	for _, line := range shared.NonEmptyLines(clean) {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "naming to ") || strings.Contains(trimmed, "writing image ") {
			out = append(out, shared.Clip(trimmed, 160))
		}
		if len(out) >= maxLines {
			break
		}
	}
	return shared.UniqueStrings(out)
}

func summarizeBuildKitResult(clean string, maxLines int) buildSystemSummaryResult {
	scan := newBuildKitScan()
	for _, line := range shared.NonEmptyLines(clean) {
		scan.ingestLine(strings.TrimSpace(line))
	}
	out := scan.successOutput(clean, maxLines)
	if scan.failed() {
		out = scan.failureOutput()
	}
	result := buildSystemSummaryResult{Text: shared.JoinLimitedLines(out, maxLines)}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}
