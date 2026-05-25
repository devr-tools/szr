package git

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	gitfilter "github.com/devr-tools/szr/internal/filters/git"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "git-status",
			Description:      "Condenses git working tree state into branch and file counts.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(15),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "git", "status")
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command[1:], "--short", "--porcelain", "-s") {
					return inv.Command
				}
				return append(inv.Command, "--short", "--branch")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return gitfilter.SummarizeGitStatus(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return gitfilter.NewGitStatusReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Rewrites `git status` into `git status --short --branch` unless a machine-readable mode was already requested.",
				"Extracts branch, staged, unstaged, and untracked counts with a short file preview.",
			},
		},
		{
			Name:             "git-log",
			Description:      "Prefers oneline commit output and trims the history preview.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 11)),
			LatencyBudget:    profilekit.LatencyBudget(15),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "git", "log")
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsPrefix(inv.Command[1:], "--format") || profilekit.ContainsAny(inv.Command[1:], "--oneline", "--stat", "-p") {
					return inv.Command
				}
				return append(inv.Command, "--oneline", "-n", "20")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return gitfilter.SummarizeGitLog(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return gitfilter.NewGitLogReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Injects `--oneline -n 20` for plain `git log` calls.",
				"Keeps the preview shallow so the LLM sees commit shape instead of full message bodies.",
			},
		},
		{
			Name:             "git-diff",
			Description:      "Summarizes file churn and preserves `--stat` style detail.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 9)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return profilekit.HasCommand(inv.Display, "git", "diff")
			},
			Prepare: func(inv engine.Invocation) []string {
				if profilekit.ContainsAny(inv.Command[1:], "--stat", "--numstat", "--shortstat", "--name-only", "--name-status") {
					return inv.Command
				}
				return append(inv.Command, "--stat=120,40")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return gitfilter.SummarizeGitDiff(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return gitfilter.NewGitDiffReducer(budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Biases `git diff` toward stat output instead of full hunks.",
				"Totals additions and deletions, then keeps the per-file summary lines.",
			},
		},
	}
}
