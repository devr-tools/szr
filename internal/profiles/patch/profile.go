package patch

import (
	"szr/internal/engine"
	shared "szr/internal/filters"
	patchfilter "szr/internal/filters/patch"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "patch-diff",
			Description:      "Summarizes diff and patch workflows around file churn, hunks, and apply failures.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return isPatchDiffCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return patchfilter.SummarizePatchDiff(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return patchfilter.SummarizePatchDiff(input, maxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches raw diff and patch application commands such as `diff`, `patch`, and `git apply`.",
				"Surfaces touched files, hunk counts, and apply failures before any raw hunk body noise.",
			},
		},
	}
}

func isPatchDiffCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch {
	case args[0] == "diff":
		return true
	case args[0] == "patch":
		return true
	case len(args) >= 2 && args[0] == "git" && args[1] == "apply":
		return true
	default:
		return false
	}
}
