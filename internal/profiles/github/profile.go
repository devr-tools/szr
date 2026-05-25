package github

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	ghfilter "github.com/devr-tools/szr/internal/filters/github"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "gh-pr-view",
			Description:      "Requests structured pull request metadata and summarizes file churn plus review state.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isGHCommand(inv.Display, "pr", "view")
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command, "--json", "--template", "--comments", "--web") || profilekit.ContainsPrefix(inv.Command, "--json=") || profilekit.ContainsPrefix(inv.Command, "--template=") {
					return inv.Command
				}
				return append(inv.Command, "--json", "number,title,state,isDraft,headRefName,baseRefName,reviewDecision,files")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return ghfilter.SummarizePRView(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newBufferedStdoutReducer(func(input string) string {
					return ghfilter.SummarizePRView(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Moves `gh pr view` into JSON mode unless the user already chose comments, web, or another formatter.",
				"Surfaces PR title, branch direction, review decision, and changed files ahead of long prose sections.",
			},
		},
		{
			Name:             "gh-pr-checks",
			Description:      "Summarizes `gh pr checks` around failed or pending checks without requiring raw table scans.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isGHCommand(inv.Display, "pr", "checks")
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return shared.SummarizeGenericFailure(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newBufferedCombinedReducer(func(input string) string {
					return shared.SummarizeGenericFailure(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `gh pr checks` to keep failed and pending checks visible.",
				"Uses a failure-oriented reducer so long check tables collapse toward the blocking rows.",
			},
		},
		{
			Name:             "gh-run-log",
			Description:      "Summarizes GitHub Actions raw logs by failed job and step.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isGHCommand(inv.Display, "run", "view") && hasGHRunLogFlag(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return ghfilter.SummarizeRunLog(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newBufferedCombinedReducer(func(input string) string {
					return ghfilter.SummarizeRunLog(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Activates when `gh run view` is already in raw log mode such as `--log` or `--log-failed`.",
				"Groups failures by job and step so long GitHub Actions logs collapse into repair-relevant signal.",
			},
		},
		{
			Name:             "gh-run-list",
			Description:      "Keeps the most informative `gh run list` rows instead of a long workflow history table.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isGHCommand(inv.Display, "run", "list")
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return shared.CompactLines(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newBufferedCombinedReducer(func(input string) string {
					return shared.CompactLines(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `gh run list` so the newest workflow runs stay visible without carrying the full history table.",
				"Keeps the latest informative lines and lets tee or raw output remain the escape hatch.",
			},
		},
		{
			Name:             "gh-run-view",
			Description:      "Summarizes GitHub Actions run state, failed jobs, and failing steps.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isGHCommand(inv.Display, "run", "view") && !hasGHRunLogFlag(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command, "--json", "--template", "--jq", "--web", "--log", "--log-failed") || profilekit.ContainsPrefix(inv.Command, "--json=") || profilekit.ContainsPrefix(inv.Command, "--template=") {
					return inv.Command
				}
				return append(inv.Command, "--json", "name,displayTitle,workflowName,status,conclusion,event,headBranch,jobs,url")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return ghfilter.SummarizeRunView(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newBufferedCombinedReducer(func(input string) string {
					return ghfilter.SummarizeRunView(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Requests structured run metadata when the user did not explicitly ask for raw logs or another formatter.",
				"Keeps workflow status, branch or event context, failed jobs, and failed step names visible for repair loops.",
			},
		},
	}
}
