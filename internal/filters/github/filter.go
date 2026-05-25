package github

import (
	"encoding/json"
	"fmt"
	"strings"

	shared "github.com/devr-tools/szr/internal/filters"
)

type PRView struct {
	Number         int      `json:"number"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	IsDraft        bool     `json:"isDraft"`
	HeadRefName    string   `json:"headRefName"`
	BaseRefName    string   `json:"baseRefName"`
	ReviewDecision string   `json:"reviewDecision"`
	Files          []PRFile `json:"files"`
}

type PRFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type RunView struct {
	Name         string   `json:"name"`
	DisplayTitle string   `json:"displayTitle"`
	WorkflowName string   `json:"workflowName"`
	Status       string   `json:"status"`
	Conclusion   string   `json:"conclusion"`
	Event        string   `json:"event"`
	HeadBranch   string   `json:"headBranch"`
	URL          string   `json:"url"`
	Jobs         []RunJob `json:"jobs"`
}

type RunJob struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	Steps      []RunStep `json:"steps"`
}

type RunStep struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
}

func SummarizePRView(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 10
	}
	clean := shared.StripANSI(input)
	var pr PRView
	if err := json.Unmarshal([]byte(strings.TrimSpace(clean)), &pr); err != nil || pr.Number == 0 {
		return shared.SummarizeGenericFailure(clean, maxLines)
	}

	out := []string{
		fmt.Sprintf("PR #%d %s state=%s draft=%t", pr.Number, pr.Title, strings.ToLower(pr.State), pr.IsDraft),
		fmt.Sprintf("%s -> %s review=%s", pr.HeadRefName, pr.BaseRefName, orDefault(strings.ToLower(pr.ReviewDecision), "none")),
		fmt.Sprintf("files: %d", len(pr.Files)),
	}
	for _, file := range pr.Files {
		out = append(out, shared.Clip(fmt.Sprintf("%s +%d -%d", file.Path, file.Additions, file.Deletions), 160))
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func SummarizeGHPRView(input string, maxLines int) string {
	return SummarizePRView(input, maxLines)
}

func SummarizeRunView(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}
	clean := shared.StripANSI(input)
	var run RunView
	if err := json.Unmarshal([]byte(strings.TrimSpace(clean)), &run); err == nil && (run.Name != "" || run.WorkflowName != "" || len(run.Jobs) > 0) {
		return summarizeStructuredRun(run, maxLines)
	}
	return shared.SummarizeGenericFailure(clean, maxLines)
}

func SummarizeGHRunView(input string, maxLines int) string {
	return SummarizeRunView(input, maxLines)
}

func SummarizeRunLog(input string, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 12
	}
	lines := shared.NonEmptyLines(shared.StripANSI(input))
	if len(lines) == 0 {
		return "ok"
	}

	jobMessages := map[string]map[string]int{}
	jobOrder := []string{}
	headers := []string{}
	for _, line := range lines {
		job, step, message := parseRunLogLine(line)
		if isRunLogHeader(job, step, message) {
			headers = append(headers, shared.Clip(line, 160))
			continue
		}
		if !sharedLineInteresting(message) {
			continue
		}
		if _, ok := jobMessages[job]; !ok {
			jobMessages[job] = map[string]int{}
			jobOrder = append(jobOrder, job)
		}
		entry := shared.Clip(strings.TrimSpace(step+" "+message), 160)
		jobMessages[job][strings.TrimSpace(entry)]++
	}

	out := []string{}
	if len(jobOrder) > 0 {
		out = append(out, fmt.Sprintf("jobs_with_failures: %d", len(jobOrder)))
	}
	out = append(out, shared.UniqueStrings(headers)...)
	for _, job := range jobOrder {
		for _, message := range sortedMessages(jobMessages[job]) {
			line := fmt.Sprintf("%s: %s", job, message)
			if jobMessages[job][message] > 1 {
				line = fmt.Sprintf("%s (x%d)", line, jobMessages[job][message])
			}
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return shared.SummarizeGenericFailure(strings.Join(lines, "\n"), maxLines)
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func SummarizeGHRunLog(input string, maxLines int) string {
	return SummarizeRunLog(input, maxLines)
}

func summarizeStructuredRun(run RunView, maxLines int) string {
	title := run.WorkflowName
	if title == "" {
		title = run.Name
	}
	if title == "" {
		title = run.DisplayTitle
	}
	out := []string{
		fmt.Sprintf("%s status=%s conclusion=%s", title, strings.ToLower(run.Status), orDefault(strings.ToLower(run.Conclusion), "unknown")),
	}
	if run.HeadBranch != "" || run.Event != "" {
		out = append(out, fmt.Sprintf("branch=%s event=%s", run.HeadBranch, run.Event))
	}
	failedJobs := 0
	for _, job := range run.Jobs {
		if strings.EqualFold(job.Conclusion, "failure") || strings.EqualFold(job.Status, "failure") {
			failedJobs++
			out = append(out, shared.Clip(fmt.Sprintf("job %s status=%s conclusion=%s", job.Name, strings.ToLower(job.Status), strings.ToLower(job.Conclusion)), 160))
			for _, step := range job.Steps {
				if strings.EqualFold(step.Conclusion, "failure") {
					out = append(out, shared.Clip("  step "+step.Name, 160))
				}
			}
		}
	}
	if failedJobs == 0 {
		out = append(out, fmt.Sprintf("jobs: %d", len(run.Jobs)))
	}
	if run.URL != "" {
		out = append(out, shared.Clip(run.URL, 160))
	}
	return shared.JoinLimitedLines(out, maxLines)
}

func parseRunLogLine(line string) (string, string, string) {
	if parts := strings.SplitN(line, "\t", 3); len(parts) == 3 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
	}
	fields := strings.Fields(line)
	if len(fields) >= 3 {
		return fields[0], fields[1], strings.TrimSpace(strings.Join(fields[2:], " "))
	}
	return "run", "log", strings.TrimSpace(line)
}

func isRunLogHeader(job, step, message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return job == "run" && step == "log" && (strings.Contains(lower, "gha run") || strings.Contains(lower, "workflow"))
}

func sharedLineInteresting(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(lower, "error") ||
		strings.Contains(lower, "failed") ||
		strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "exception") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "timed out")
}

func sortedMessages(counts map[string]int) []string {
	order := make([]string, 0, len(counts))
	for message := range counts {
		order = append(order, message)
	}
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if order[j] < order[i] {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	return order
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
