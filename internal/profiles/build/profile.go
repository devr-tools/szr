package build

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	buildfilter "github.com/devr-tools/szr/internal/filters/build"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "build-system",
			Description:      "Summarizes common build-orchestration tools around failing targets and actionable diagnostics.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isBuildSystemCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return buildfilter.SummarizeBuildSystem(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return buildfilter.SummarizeBuildSystem(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches common build-system entrypoints such as `make`, `just`, `task`, `bazel`, `ninja`, and `cmake`.",
				"Keeps failing targets, build-system error lines, and source file anchors instead of raw parallel build chatter.",
			},
		},
	}
}

func isBuildSystemCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "make", "just", "task", "bazel", "ninja", "cmake", "terraform", "tofu", "helm", "gradle", "mvn":
		return true
	case "docker":
		return len(args) >= 2 && (args[1] == "build" || args[1] == "buildx" && len(args) >= 3 && args[2] == "build")
	default:
		return false
	}
}
