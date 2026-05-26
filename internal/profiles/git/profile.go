package git

import (
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	gitfilter "github.com/devr-tools/szr/internal/filters/git"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	statusSummary := profilekit.StdoutSummary(maxLines, 8, 15, engine.StreamStdoutOnly, func(stdout string) string {
		return gitfilter.SummarizeGitStatus(shared.StripANSI(stdout))
	}, func(budget engine.OutputBudget) engine.StreamReducer {
		return gitfilter.NewGitStatusReducer(budget.MaxLines, budget.MaxBytes)
	})
	logSummary := profilekit.StdoutSummary(maxLines, 11, 15, engine.StreamStdoutOnly, func(stdout string) string {
		return gitfilter.SummarizeGitLog(shared.StripANSI(stdout))
	}, func(budget engine.OutputBudget) engine.StreamReducer {
		return gitfilter.NewGitLogReducer(budget.MaxLines, budget.MaxBytes)
	})

	return []engine.Profile{
		{
			Name:        "git-ls-files",
			Description: "Summarizes tracked file lists into a bounded path preview.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
			},
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(15),
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "ls-files"
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return shared.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewFindReducer(budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `git ls-files` directly instead of routing file lists through a generic fallback.",
				"Summarizes tracked paths as a bounded, deduplicated list with the same path-list behavior used for other discovery commands.",
			},
		},
		profilekit.WithSummary(engine.Profile{
			Name:        "git-status",
			Description: "Condenses git working tree state into branch and file counts.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:            engine.FastPathBypassSafeOnly,
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "status"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.StatusFormatRequested {
					return inv.Command
				}
				return append(inv.Command, "--short", "--branch")
			},
			Explain: []string{
				"Rewrites `git status` into `git status --short --branch` unless a machine-readable mode was already requested.",
				"Extracts branch, staged, unstaged, and untracked counts with a short file preview.",
			},
		}, statusSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "git-log",
			Description: "Prefers oneline commit output and trims the history preview.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "log"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.LogFormatRequested {
					return inv.Command
				}
				return append(inv.Command, "--oneline", "-n", "20")
			},
			Explain: []string{
				"Injects `--oneline -n 20` for plain `git log` calls.",
				"Keeps the preview shallow so the LLM sees commit shape instead of full message bodies.",
			},
		}, logSummary),
		{
			Name:        "git-diff",
			Description: "Summarizes file churn and preserves `--stat` style detail.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
			},
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 9)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "diff"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.DiffNoPatchRequested {
					return inv.Command
				}
				if !inv.Advanced.AggressivePrepareRewrites {
					if inv.Classification.Command.Git.DiffFormatRequested {
						return inv.Command
					}
					return append(inv.Command, "--stat=120,40")
				}
				if inv.Classification.Command.Git.DiffFormatRequested {
					return ensureGitDiffNoiseFlags(inv.Command)
				}
				if isAggressiveGitDiff(inv) {
					return ensureGitDiffNoiseFlags(append(inv.Command, "--stat=72,12", "--compact-summary"))
				}
				return ensureGitDiffNoiseFlags(append(inv.Command, "--stat=96,24", "--compact-summary"))
			},
			Render: func(inv engine.Invocation, exec engine.Execution) string {
				return newGitDiffReducer(inv, maxLines, 0).Reduce(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newGitDiffReducer(inv, budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Biases `git diff` toward stat output instead of full hunks, with narrower stat widths in aggressive mode.",
				"Totals additions and deletions, then keeps the highest-churn files when the diff touches many paths.",
			},
		},
	}
}

func newGitDiffReducer(inv engine.Invocation, maxLines int, maxBytes int) *gitfilter.GitDiffReducer {
	return gitfilter.NewGitDiffReducerWithOptions(gitfilter.GitDiffReducerOptions{
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
		Aggressive:            isAggressiveGitDiff(inv),
		LargeFileThreshold:    8,
		LargeSummaryTopN:      5,
		AggressiveSummaryTopN: 3,
	})
}

func isAggressiveGitDiff(inv engine.Invocation) bool {
	return inv.UltraCompact || inv.ReasoningBudgetMode == config.ReasoningBudgetAggressive
}

func ensureGitDiffNoiseFlags(command []string) []string {
	out := append([]string{}, command...)
	if !profilekit.ContainsAny(command[1:], "--no-color", "--color=never") && !profilekit.ContainsPrefix(command[1:], "--color=") {
		out = append(out, "--no-color")
	}
	if !profilekit.ContainsAny(command[1:], "--no-ext-diff", "--ext-diff") {
		out = append(out, "--no-ext-diff")
	}
	return out
}
