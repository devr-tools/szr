package container

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	containerfilter "github.com/devr-tools/szr/internal/filters/container"
	"github.com/devr-tools/szr/internal/profilekit"
)

func dockerImagesProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "docker-images",
		Description:      "Summarizes `docker images` listings into image totals plus compact repo:tag, size, and age lines.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match: func(inv engine.Invocation) bool {
			return matchDockerImages(inv.Display)
		},
		Prepare: func(inv engine.Invocation) []string {
			return prepareDockerImagesCommand(inv.Command)
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return containerfilter.SummarizeDockerImages(exec.Stdout, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(
				true,
				false,
				func(input string) string {
					return containerfilter.SummarizeDockerImages(input, budget.MaxLines)
				},
				func(input string) (string, string, bool) {
					return containerfilter.DockerImagesRecoveryInfo(input, budget.MaxLines)
				},
			)
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Moves `docker images`/`docker image ls` to a tab format unless the user already chose --format, --quiet, or --digests.",
			"Leads with image count and size total, collapses dangling `<none>` images into one line, and keeps repo:tag, size, and age per image.",
		},
	}
}

func matchDockerImages(args []string) bool {
	return profilekit.HasCommand(args, "docker", "images") || isDockerImageList(args)
}

func isDockerImageList(args []string) bool {
	return len(args) >= 3 && args[0] == "docker" && args[1] == "image" && (args[2] == "ls" || args[2] == "list")
}

func prepareDockerImagesCommand(command []string) []string {
	var args []string
	switch {
	case profilekit.HasCommand(command, "docker", "images"):
		args = command[2:]
	case isDockerImageList(command):
		args = command[3:]
	default:
		return command
	}
	if profilekit.ContainsAny(args, "--format", "-q", "--quiet", "--digests") || profilekit.ContainsPrefix(args, "--format=") {
		return command
	}
	return append(command, "--format", "{{.Repository}}\t{{.Tag}}\t{{.Size}}\t{{.CreatedSince}}")
}
