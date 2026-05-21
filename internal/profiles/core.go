package profiles

import (
	"strings"

	"szr/internal/engine"
	"szr/internal/filters"
)

func coreProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:        "git-status",
			Description: "Condenses git working tree state into branch and file counts.",
			Confidence:  engine.ConfidenceHigh,
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "git", "status")
			},
			Prepare: func(inv engine.Invocation) []string {
				if containsAny(inv.Command[1:], "--short", "--porcelain", "-s") {
					return inv.Command
				}
				return append(inv.Command, "--short", "--branch")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGitStatus(filters.StripANSI(exec.Stdout))
			},
			ParseBytes: parseStdout,
			Explain: []string{
				"Rewrites `git status` into `git status --short --branch` unless a machine-readable mode was already requested.",
				"Extracts branch, staged, unstaged, and untracked counts with a short file preview.",
			},
		},
		{
			Name:        "git-log",
			Description: "Prefers oneline commit output and trims the history preview.",
			Confidence:  engine.ConfidenceHigh,
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "git", "log")
			},
			Prepare: func(inv engine.Invocation) []string {
				if containsPrefix(inv.Command[1:], "--format") || containsAny(inv.Command[1:], "--oneline", "--stat", "-p") {
					return inv.Command
				}
				return append(inv.Command, "--oneline", "-n", "20")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGitLog(filters.StripANSI(exec.Stdout))
			},
			ParseBytes: parseStdout,
			Explain: []string{
				"Injects `--oneline -n 20` for plain `git log` calls.",
				"Keeps the preview shallow so the LLM sees commit shape instead of full message bodies.",
			},
		},
		{
			Name:        "git-diff",
			Description: "Summarizes file churn and preserves `--stat` style detail.",
			Confidence:  engine.ConfidenceHigh,
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "git", "diff")
			},
			Prepare: func(inv engine.Invocation) []string {
				if containsAny(inv.Command[1:], "--stat", "--numstat", "--shortstat", "--name-only", "--name-status") {
					return inv.Command
				}
				return append(inv.Command, "--stat=120,40")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGitDiff(filters.StripANSI(exec.Stdout))
			},
			ParseBytes: parseStdout,
			Explain: []string{
				"Biases `git diff` toward stat output instead of full hunks.",
				"Totals additions and deletions, then keeps the per-file summary lines.",
			},
		},
		{
			Name:        "go-test-json",
			Description: "Forces `go test -json` and reports package-level failures only.",
			Confidence:  engine.ConfidenceHigh,
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "go", "test")
			},
			Prepare: func(inv engine.Invocation) []string {
				if containsAny(inv.Command[1:], "-json") {
					return inv.Command
				}
				return append(inv.Command, "-json")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGoTestJSON(exec.Stdout)
			},
			ParseBytes: parseStdout,
			Explain: []string{
				"Upgrades `go test` to NDJSON mode.",
				"Collapses passing noise and keeps failed packages, tests, and panic lines.",
			},
		},
		{
			Name:        "go-build",
			Description: "Drops download noise and focuses on compiler diagnostics.",
			Confidence:  engine.ConfidenceHigh,
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "go", "build") || hasCommand(inv.Display, "go", "vet")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGenericFailure(exec.Stderr+"\n"+exec.Stdout, maxLines)
			},
			ParseBytes: parseStderrFirst,
			Explain: []string{
				"Treats stderr as primary signal for compiler and vet output.",
				"Surfaces error-bearing lines first and trims boilerplate.",
			},
		},
		{
			Name:        "generic-test",
			Description: "Generic failure-focused profile for wrapped test commands.",
			Confidence:  engine.ConfidenceMedium,
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "test"
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeGenericFailure(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			ParseBytes: parseCombined,
			Explain: []string{
				"Uses a keyword-focused fallback for arbitrary test runners.",
				"Best-effort mode when there is no structured parser for the tool.",
			},
		},
		{
			Name:        "generic-summary",
			Description: "Keeps the first informative lines from long command output.",
			Confidence:  engine.ConfidenceLow,
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "summary"
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.CompactLines(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			ParseBytes: parseCombined,
			Explain: []string{
				"Does not attempt tool-specific parsing.",
				"Useful when the user wants a shallow preview before drilling deeper.",
			},
		},
	}
}

func hasCommand(args []string, head, sub string) bool {
	return len(args) >= 2 && args[0] == head && args[1] == sub
}

func containsAny(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func containsPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}
