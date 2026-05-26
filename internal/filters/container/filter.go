package container

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

func SummarizeDockerPS(input string, maxLines int) string {
	return summarizeDockerPSResult(input, maxLines).Text
}

func DockerPSRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDockerPSResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional containers", result.OmittedCount))
}

type dockerPSSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeDockerPSResult(input string, maxLines int) dockerPSSummaryResult {
	if maxLines <= 0 {
		maxLines = 10
	}

	clean := shared.StripANSI(input)
	if services := parseComposePSJSON(clean); len(services) > 0 {
		return summarizeDockerServices(services, maxLines)
	}

	lines := shared.NonEmptyLines(clean)
	if len(lines) == 0 {
		return dockerPSSummaryResult{Text: "ok"}
	}

	services := make([]dockerServiceState, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		services = append(services, dockerServiceState{
			Name:   strings.TrimSpace(parts[0]),
			State:  strings.TrimSpace(parts[1]),
			Image:  strings.TrimSpace(parts[2]),
			Source: "docker",
		})
	}
	if len(services) > 0 {
		return summarizeDockerServices(services, maxLines)
	}
	return dockerPSSummaryResult{Text: shared.CompactLines(clean, maxLines)}
}

func SummarizeDockerLogs(input string, maxLines int) string {
	return summarizeDockerLogsResult(input, maxLines).Text
}

func DockerLogsRecoveryInfo(input string, maxLines int) (string, string, bool) {
	result := summarizeDockerLogsResult(input, maxLines)
	if result.OmittedCount <= 0 {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery(fmt.Sprintf("omitted %d additional log lines", result.OmittedCount))
}

type dockerLogsSummaryResult struct {
	Text         string
	OmittedCount int
}

func summarizeDockerLogsResult(input string, maxLines int) dockerLogsSummaryResult {
	if maxLines <= 0 {
		maxLines = 12
	}

	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return dockerLogsSummaryResult{Text: "ok"}
	}

	type sourceSummary struct {
		count    int
		order    []string
		seen     map[string]int
		headLine string
	}

	sources := map[string]*sourceSummary{}
	sourceOrder := []string{}
	getSource := func(name string) *sourceSummary {
		if existing, ok := sources[name]; ok {
			return existing
		}
		entry := &sourceSummary{seen: map[string]int{}}
		sources[name] = entry
		sourceOrder = append(sourceOrder, name)
		return entry
	}

	for _, line := range lines {
		source, message := splitDockerLogLine(line)
		entry := getSource(source)
		entry.count++
		if entry.headLine == "" {
			entry.headLine = shared.Clip(strings.TrimSpace(message), 160)
		}
		if !isInterestingLogLine(message) {
			continue
		}
		normalized := shared.Clip(strings.TrimSpace(message), 160)
		if _, ok := entry.seen[normalized]; !ok {
			entry.order = append(entry.order, normalized)
		}
		entry.seen[normalized]++
	}

	out := []string{fmt.Sprintf("sources: %d", len(sourceOrder))}
	for _, source := range sourceOrder {
		entry := sources[source]
		if entry == nil {
			continue
		}
		if len(entry.order) == 0 {
			out = append(out, fmt.Sprintf("%s: %s", source, entry.headLine))
			continue
		}
		for _, message := range entry.order {
			line := fmt.Sprintf("%s: %s", source, message)
			if entry.seen[message] > 1 {
				line = fmt.Sprintf("%s (x%d)", line, entry.seen[message])
			}
			out = append(out, line)
		}
	}
	result := dockerLogsSummaryResult{
		Text: shared.JoinLimitedLines(out, maxLines),
	}
	if len(out) > maxLines {
		result.OmittedCount = len(out) - maxLines
	}
	return result
}

type dockerServiceState struct {
	Name   string
	State  string
	Image  string
	Health string
	Source string
}

type composePSItem struct {
	Name    string `json:"Name"`
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
	Image   string `json:"Image"`
}

func parseComposePSJSON(input string) []dockerServiceState {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || trimmed[0] != '[' {
		return nil
	}
	var items []composePSItem
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	out := make([]dockerServiceState, 0, len(items))
	for _, item := range items {
		name := item.Service
		if name == "" {
			name = item.Name
		}
		out = append(out, dockerServiceState{
			Name:   name,
			State:  item.State,
			Image:  item.Image,
			Health: item.Health,
			Source: "compose",
		})
	}
	return out
}

func summarizeDockerServices(services []dockerServiceState, maxLines int) dockerPSSummaryResult {
	if len(services) == 0 {
		return dockerPSSummaryResult{Text: "ok"}
	}
	running := 0
	exited := 0
	other := 0
	out := []string{}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, service := range services {
		state := strings.TrimSpace(service.State)
		normalized := strings.ToLower(state)
		switch {
		case strings.Contains(normalized, "running") || normalized == "running" || strings.HasPrefix(normalized, "up ") || normalized == "up":
			running++
		case strings.Contains(normalized, "exited") || normalized == "exited":
			exited++
		default:
			other++
		}
		line := fmt.Sprintf("%s: %s", service.Name, state)
		if service.Health != "" {
			line += " (" + service.Health + ")"
		}
		if service.Image != "" {
			line += " [" + service.Image + "]"
		}
		out = append(out, shared.Clip(line, 160))
	}
	header := fmt.Sprintf("containers: running=%d exited=%d other=%d", running, exited, other)
	lines := append([]string{header}, out...)
	result := dockerPSSummaryResult{
		Text: shared.JoinLimitedLines(lines, maxLines),
	}
	if len(lines) > maxLines {
		result.OmittedCount = len(lines) - maxLines
	}
	return result
}

func splitDockerLogLine(line string) (string, string) {
	if before, after, ok := strings.Cut(line, " | "); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	return "log", strings.TrimSpace(line)
}

func isInterestingLogLine(line string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(line))
	switch {
	case trimmed == "":
		return false
	case strings.Contains(trimmed, "error"),
		strings.Contains(trimmed, "warn"),
		strings.Contains(trimmed, "panic"),
		strings.Contains(trimmed, "fatal"),
		strings.Contains(trimmed, "failed"),
		strings.Contains(trimmed, "exception"),
		strings.Contains(trimmed, "timeout"),
		strings.Contains(trimmed, "refused"),
		strings.Contains(trimmed, "unhealthy"),
		strings.Contains(trimmed, "traceback"):
		return true
	default:
		return false
	}
}
