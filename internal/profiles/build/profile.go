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
				if !inv.Advanced.AggressivePrepareRewrites {
					return inv.Command
				}
				return prepareBuildSystemCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return buildfilter.SummarizeBuildSystem(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					true,
					func(input string) string {
						return buildfilter.SummarizeBuildSystem(input, budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return buildfilter.BuildSystemRecoveryInfo(input, budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches common build-system entrypoints such as `make`, `just`, `task`, `bazel`, `ninja`, and `cmake`.",
				"Keeps failing targets, build-system error lines, and source file anchors instead of raw parallel build chatter.",
				"Recognizes terraform/tofu plan and apply output, keeping the `Plan:`/`Apply complete!` summary, error and warning blocks, and resource headers while dropping attribute-level diff noise.",
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

func prepareBuildSystemCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	switch command[0] {
	case "make":
		out = prepareMakeCommand(out, command[1:])
	case "bazel":
		out = prepareBazelCommand(out, command[1:])
	case "terraform", "tofu", "helm":
		out = appendBuildArgIfMissing(out, command[1:], "-no-color")
	case "gradle":
		out = prepareGradleCommand(out, command[1:])
	case "mvn":
		out = prepareMavenCommand(out, command[1:])
	}
	return out
}

func prepareMakeCommand(out, args []string) []string {
	if !profilekit.ContainsAny(args, "--no-print-directory", "-s", "--silent") {
		out = append(out, "--no-print-directory")
	}
	return out
}

func prepareBazelCommand(out, args []string) []string {
	out = appendBuildArgIfMissing(out, args, "--noshow_progress")
	if !profilekit.ContainsAny(args, "--color=no") && !profilekit.ContainsPrefix(args, "--color=") {
		out = append(out, "--color=no")
	}
	if !profilekit.ContainsAny(args, "--curses=no") && !profilekit.ContainsPrefix(args, "--curses=") {
		out = append(out, "--curses=no")
	}
	return out
}

func prepareGradleCommand(out, args []string) []string {
	if !profilekit.ContainsAny(args, "-q", "--quiet", "-i", "--info", "-d", "--debug") {
		out = append(out, "--quiet")
	}
	if !profilekit.ContainsPrefix(args, "--console=") {
		out = append(out, "--console=plain")
	}
	return out
}

func prepareMavenCommand(out, args []string) []string {
	if !profilekit.ContainsAny(args, "-q", "--quiet", "-X", "--debug", "-e", "--errors") {
		out = append(out, "--quiet")
	}
	if !profilekit.ContainsAny(args, "--batch-mode", "-B") {
		out = append(out, "--batch-mode")
	}
	return out
}

func appendBuildArgIfMissing(out, args []string, arg string) []string {
	if !profilekit.ContainsAny(args, arg) {
		out = append(out, arg)
	}
	return out
}
