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

func summarizeDockerTransferResult(input string, maxLines int) containerSummaryResult {
	if maxLines <= 0 {
		maxLines = 8
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return containerSummaryResult{Text: "ok"}
	}

	layerStatus := map[string]string{}
	pushSeen := false
	errorLines := []string{}
	kept := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := dockerLayerLinePattern.FindString(trimmed); match != "" {
			id := strings.TrimSuffix(match, ": ")
			status := strings.TrimSpace(strings.TrimPrefix(trimmed, match))
			layerStatus[id] = status
			lowerStatus := strings.ToLower(status)
			if strings.Contains(lowerStatus, "error") || strings.Contains(lowerStatus, "failed") {
				errorLines = append(errorLines, shared.Clip(trimmed, 160))
			}
			continue
		}
		if strings.Contains(trimmed, "The push refers to") {
			pushSeen = true
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.Contains(lower, "error") ||
			strings.Contains(lower, "denied") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "manifest unknown"):
			errorLines = append(errorLines, shared.Clip(trimmed, 160))
		case strings.Contains(trimmed, "Pulling from "),
			strings.Contains(trimmed, "The push refers to"),
			strings.HasPrefix(trimmed, "Digest:"),
			strings.HasPrefix(trimmed, "Status:"),
			strings.Contains(lower, "digest: sha256:"):
			kept = append(kept, shared.Clip(trimmed, 160))
		}
	}

	errorLines = shared.UniqueStrings(errorLines)
	kept = shared.UniqueStrings(kept)
	if len(layerStatus) == 0 && len(errorLines) == 0 && len(kept) == 0 {
		return containerSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}

	out := []string{}
	if len(layerStatus) > 0 {
		existing := 0
		for _, status := range layerStatus {
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
		layerLine := fmt.Sprintf("%s %d layers", verb, len(layerStatus)-existing)
		if existing > 0 {
			layerLine += fmt.Sprintf(" (%d already existed)", existing)
		}
		out = append(out, layerLine)
	}
	out = append(out, errorLines...)
	out = append(out, kept...)
	return limitContainerLines(out, maxLines)
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

func summarizeComposeActivityResult(input string, maxLines int) containerSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	clean := shared.StripANSI(input)
	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return containerSummaryResult{Text: "ok"}
	}

	states := map[string]string{}
	order := []string{}
	startedEvents := 0
	healthyEvents := 0
	errorLines := []string{}
	attach := []string{}
	lastProgress := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := composeResourceStatePattern.FindStringSubmatch(trimmed); match != nil {
			key := match[1] + " " + match[2]
			state := strings.TrimSpace(match[3])
			if _, seen := states[key]; !seen {
				order = append(order, key)
			}
			states[key] = state
			switch {
			case strings.HasPrefix(state, "Started"):
				startedEvents++
			case strings.HasPrefix(state, "Healthy"):
				healthyEvents++
			}
			continue
		}
		if strings.HasPrefix(trimmed, "[+]") {
			lastProgress = trimmed
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "fatal") ||
			strings.Contains(lower, "panic") ||
			strings.Contains(lower, "exit code") ||
			strings.Contains(lower, "did not complete successfully") {
			errorLines = append(errorLines, shared.Clip(trimmed, 160))
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// BuildKit progress lines; failures were already captured above.
			continue
		}
		if source, message, ok := strings.Cut(line, " | "); ok {
			if isInterestingLogLine(message) {
				attach = append(attach, shared.Clip(strings.TrimSpace(source)+": "+strings.TrimSpace(message), 160))
			}
			continue
		}
	}

	errorLines = shared.UniqueStrings(errorLines)
	attach = shared.UniqueStrings(attach)
	if len(order) == 0 && len(errorLines) == 0 && len(attach) == 0 {
		if lastProgress != "" {
			return limitContainerLines([]string{shared.Clip(lastProgress, 160)}, maxLines)
		}
		return containerSummaryResult{Text: shared.CompactLines(clean, maxLines)}
	}

	out := []string{}
	if len(order) > 0 {
		out = append(out, fmt.Sprintf("services: started=%d healthy=%d", startedEvents, healthyEvents))
	}
	out = append(out, errorLines...)
	for _, key := range order {
		state := states[key]
		if composeIntermediateState(state) {
			continue
		}
		out = append(out, shared.Clip(key+" "+state, 160))
	}
	out = append(out, attach...)
	if len(out) == 0 && lastProgress != "" {
		out = append(out, shared.Clip(lastProgress, 160))
	}
	return limitContainerLines(out, maxLines)
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
