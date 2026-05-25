package profiles

import (
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

func coreProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		directoryListingProfile(maxLines),
		catReadProfile(maxLines),
		goTestJSONProfile(maxLines),
		goBuildProfile(maxLines),
		genericTestProfile(maxLines),
		genericSummaryProfile(maxLines),
	}
}

func directoryListingProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "directory-listing",
		Description:      "Condenses ls and tree output into a short directory preview.",
		Confidence:       engine.ConfidenceMedium,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(15),
		Match: func(inv engine.Invocation) bool {
			return len(inv.Command) > 0 && (inv.Command[0] == "ls" || inv.Command[0] == "tree")
		},
		Prepare: func(inv engine.Invocation) []string {
			if len(inv.Command) == 0 {
				return inv.Command
			}
			switch inv.Command[0] {
			case "ls":
				return prepareLSCommand(inv.Command)
			case "tree":
				return prepareTreeCommand(inv.Command)
			default:
				return inv.Command
			}
		},
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			if len(inv.Command) > 0 && inv.Command[0] == "tree" {
				return filters.SummarizeTreeOutput(exec.Stdout, maxLines)
			}
			return filters.SummarizeDirectoryListing(exec.Stdout, maxLines)
		},
		StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewBufferedTextReducer(true, false, func(input string) string {
				if len(inv.Command) > 0 && inv.Command[0] == "tree" {
					return filters.SummarizeTreeOutput(input, budget.MaxLines)
				}
				return filters.SummarizeDirectoryListing(input, budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Normalizes plain `ls` into one-entry-per-line output and caps plain `tree` depth when the user did not already choose a layout.",
			"Summarizes directory listings as compact directory and file groups instead of replaying every entry.",
		},
	}
}

func catReadProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "cat-read",
		Description:      "Turns single-file cat output into a structural preview.",
		Confidence:       engine.ConfidenceMedium,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match: func(inv engine.Invocation) bool {
			return isSingleFileCat(inv.Command)
		},
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeReadFile(inv.Command[len(inv.Command)-1], []byte(exec.Stdout), maxLines)
		},
		StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			path := inv.Command[len(inv.Command)-1]
			return filters.NewBufferedTextReducer(true, false, func(input string) string {
				return filters.SummarizeReadFile(path, []byte(input), budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Matches simple single-file `cat` usage without flags.",
			"Prefers headings, declarations, and anchored lines over replaying the whole file verbatim.",
		},
	}
}

func goTestJSONProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "go-test-json",
		Description:      "Forces `go test -json` and reports package-level failures only.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(35),
		Match: func(inv engine.Invocation) bool {
			return profilekit.HasCommand(inv.Command, "go", "test") || profilekit.HasCommand(inv.Display, "go", "test")
		},
		Prepare: func(inv engine.Invocation) []string {
			if profilekit.ContainsAny(inv.Command[1:], "-json") {
				return inv.Command
			}
			return append(inv.Command, "-json")
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeGoTestJSON(exec.Stdout)
		},
		StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
			return filters.NewBufferedTextReducer(true, false, func(input string) string {
				return filters.SummarizeGoTestJSON(input)
			})
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Upgrades `go test` to NDJSON mode.",
			"Collapses passing noise and keeps failed packages, tests, and panic lines.",
		},
	}
}

func goBuildProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "go-build",
		Description:      "Drops download noise and focuses on compiler diagnostics.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStderrFirst,
		Budget:           engine.OutputBudget{MaxLines: profilekit.AtLeast(maxLines, 10), MaxBytes: profilekit.AtLeast(maxLines, 10) * 160, MaxTokens: profilekit.AtLeast(maxLines, 10) * 32, MinFailures: 1, MinAnchors: 1, MinHints: 1},
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match: func(inv engine.Invocation) bool {
			return profilekit.HasCommand(inv.Command, "go", "build") ||
				profilekit.HasCommand(inv.Command, "go", "vet") ||
				profilekit.HasCommand(inv.Display, "go", "build") ||
				profilekit.HasCommand(inv.Display, "go", "vet")
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeGenericFailure(exec.Stderr+"\n"+exec.Stdout, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewGenericFailureReducerWithContract(budget.MaxLines, budget.MaxBytes, budget.MinFailures, budget.MinAnchors, budget.MinHints)
		},
		ParseBytes: profilekit.ParseStderrFirst,
		Explain: []string{
			"Treats stderr as primary signal for compiler and vet output.",
			"Surfaces error-bearing lines first and trims boilerplate.",
		},
	}
}

func genericTestProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "generic-test",
		Description:      "Generic failure-focused profile for wrapped test commands.",
		Confidence:       engine.ConfidenceMedium,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           engine.OutputBudget{MaxLines: profilekit.AtLeast(maxLines, 8), MaxBytes: profilekit.AtLeast(maxLines, 8) * 160, MaxTokens: profilekit.AtLeast(maxLines, 8) * 32, MinFailures: 1, MinAnchors: 1, MinHints: 1},
		LatencyBudget:    profilekit.LatencyBudget(35),
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "test" && !isWrappedSpecializedTest(inv.Command)
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.SummarizeGenericFailure(exec.Stdout+"\n"+exec.Stderr, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewGenericFailureReducerWithContract(budget.MaxLines, budget.MaxBytes, budget.MinFailures, budget.MinAnchors, budget.MinHints)
		},
		ParseBytes: profilekit.ParseCombined,
		Explain: []string{
			"Uses a keyword-focused fallback for arbitrary test runners.",
			"Best-effort mode when there is no structured parser for the tool.",
		},
	}
}

func genericSummaryProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "generic-summary",
		Description:      "Keeps the first informative lines from long command output.",
		Confidence:       engine.ConfidenceLow,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "summary"
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return filters.CompactLines(exec.Stdout+"\n"+exec.Stderr, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return filters.NewCompactLineReducer(budget.MaxLines, budget.MaxBytes)
		},
		ParseBytes: profilekit.ParseCombined,
		Explain: []string{
			"Does not attempt tool-specific parsing.",
			"Useful when the user wants a shallow preview before drilling deeper.",
		},
	}
}

func isSingleFileCat(command []string) bool {
	if len(command) != 2 || command[0] != "cat" {
		return false
	}
	return command[1] != "" && command[1][0] != '-'
}

func prepareLSCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	out := append([]string{}, command...)
	if !profilekit.ContainsAny(out[1:], "-1", "-l", "-m", "-x", "-C") && !containsLSFormatFlag(out[1:]) {
		out = append(out, "-1")
	}
	if !profilekit.ContainsAny(out[1:], "-p", "-F") && !profilekit.ContainsPrefix(out[1:], "--classify") && !profilekit.ContainsPrefix(out[1:], "--indicator-style") {
		out = append(out, "-p")
	}
	return out
}

func prepareTreeCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	out := append([]string{}, command...)
	if !profilekit.ContainsAny(out[1:], "-L") && !profilekit.ContainsPrefix(out[1:], "-L") && !profilekit.ContainsPrefix(out[1:], "--level") {
		out = append(out, "-L", "2")
	}
	return out
}

func containsLSFormatFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--format=single-column" || arg == "--format=long" || arg == "--format=across" {
			return true
		}
	}
	return false
}

func isWrappedSpecializedTest(command []string) bool {
	if len(command) < 2 {
		return false
	}
	if profilekit.HasCommand(command, "go", "test") || profilekit.HasCommand(command, "cargo", "test") || profilekit.HasCommand(command, "bun", "test") {
		return true
	}
	if command[0] == "vitest" || command[0] == "jest" || command[0] == "pytest" {
		return true
	}
	if len(command) >= 3 && command[0] == "uv" && command[1] == "run" && command[2] == "pytest" {
		return true
	}
	return isPackageManagerTestLike(command)
}

func isPackageManagerTestLike(args []string) bool {
	return len(args) >= 2 &&
		(args[0] == "npm" || args[0] == "pnpm" || args[0] == "yarn") &&
		(args[1] == "test" || len(args) >= 3 && args[1] == "run" && args[2] == "test")
}
