package search

import (
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int, maxGroups int) []engine.Profile {
	ripgrepSummary := profilekit.SummaryConfig{
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    25,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeRipgrep(exec.Stdout+"\n"+exec.Stderr, maxGroups, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewRipgrepReducer(maxGroups, budget.MaxLines)
		},
		ParseBytes: profilekit.ParseCombined,
	}
	// Stdin-mode grep output has no path:line: prefixes, so the grouped
	// ripgrep reducer would drop every line and report "no matches".
	// Route those invocations to a compact line summary instead.
	grepSummary := ripgrepSummary
	grepSummary.Render = func(inv engine.Invocation, exec engine.Execution) string {
		if isStdinGrepInvocation(inv) {
			return filters.CompactLines(exec.Stdout+"\n"+exec.Stderr, maxLines)
		}
		return filters.SummarizeRipgrep(exec.Stdout+"\n"+exec.Stderr, maxGroups, maxLines)
	}
	grepSummary.StreamRender = func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
		if isStdinGrepInvocation(inv) {
			return filters.NewCompactLineReducer(budget.MaxLines, 0)
		}
		return filters.NewRipgrepReducer(maxGroups, budget.MaxLines)
	}
	pathListSummary := profilekit.SummaryConfig{
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
		LatencyBudget:    20,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewFindReducer(budget.MaxLines)
		},
		ParseBytes: profilekit.ParseCombined,
	}

	return []engine.Profile{
		profilekit.WithSummary(engine.Profile{
			Name:        "grep",
			Description: "Normalizes recursive grep into stable line-oriented output and groups matches by file; compacts stdin-filter grep output.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				RequireFullCapture: true,
				BenignExitCodes:    []int{1},
			},
			Match: func(inv engine.Invocation) bool {
				if inv.Classification.Display.Head != "grep" {
					return false
				}
				return isRecursiveGrepCommand(inv.Display) || isStdinGrepCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareGrep(inv.Command)
			},
			Explain: []string{
				"Targets recursive plain-text `grep` usage and adds filename plus line-number flags when the user did not already request them.",
				"Also targets pattern-only grep pipeline filters reading stdin, leaving their arguments untouched and compacting the matched lines instead of grouping by file.",
				"Groups matches by file and keeps grep as a viable fallback when `rg` is unavailable.",
			},
		}, grepSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "ripgrep-files",
			Description: "Summarizes `rg --files` output into a bounded path list.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				RequireFullCapture: true,
				BenignExitCodes:    []int{1},
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "rg" && isRipgrepFilesCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrepFiles(inv.Command)
			},
			Explain: []string{
				"Targets `rg --files` path-list output and preserves ripgrep ignore behavior unless the user explicitly overrides it.",
				"Summarizes tracked paths as a bounded, deduplicated match list instead of treating the command as a generic fallback.",
			},
		}, pathListSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "ripgrep-files-with-matches",
			Description: "Summarizes `rg --files-with-matches` output into a bounded path list.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				RequireFullCapture: true,
				BenignExitCodes:    []int{1},
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "rg" && isRipgrepFilesWithMatchesCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrepFiles(inv.Command)
			},
			Explain: []string{
				"Targets `rg --files-with-matches` path-list output while preserving ripgrep ignore behavior unless the user explicitly overrides it.",
				"Summarizes matching file paths as a bounded, deduplicated list instead of routing through a generic fallback.",
			},
		}, pathListSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "ripgrep",
			Description: "Normalizes ripgrep into stable line-oriented output and groups matches by file.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				RequireFullCapture: true,
				BenignExitCodes:    []int{1},
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "rg" && isRipgrepCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrep(inv.Command)
			},
			Explain: []string{
				"Targets plain-text `rg` usage and adds filename plus line-number flags when the user did not already request them.",
				"Groups matches by file and falls back to error-focused output when ripgrep fails instead of returning matches.",
			},
		}, ripgrepSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "path-find",
			Description: "Summarizes plain `find` output into a bounded match list.",
			Confidence:  engine.ConfidenceMedium,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				RequireFullCapture: true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "find" && isPlainFindCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareFind(inv.Command)
			},
			Explain: []string{
				"Targets plain `find` usage that emits path lists without destructive actions or custom executors.",
				"Caps long file discovery output to a counted preview instead of replaying every path line.",
			},
		}, pathListSummary),
	}
}
