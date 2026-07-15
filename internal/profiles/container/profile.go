package container

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	containerfilter "github.com/devr-tools/szr/internal/filters/container"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		dockerPSProfile(maxLines),
		dockerImagesProfile(maxLines),
		dockerLogsProfile(maxLines),
		dockerTransferProfile(maxLines),
		dockerActivityProfile(maxLines),
	}
}

var dockerTransferExplain = []string{
	"Collapses per-layer `Downloading`/`Extracting`/`Pull complete` progress into a single layer count.",
	"Keeps the image header, digest, final status, and any registry errors such as denied or manifest-unknown responses.",
}

func matchDockerTransfer(inv engine.Invocation) bool {
	return profilekit.HasCommand(inv.Display, "docker", "pull") ||
		profilekit.HasCommand(inv.Display, "docker", "push") ||
		isDockerComposeCommand(inv.Display, "pull") ||
		isDockerComposeCommand(inv.Display, "push")
}

func renderDockerTransfer(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(_ engine.Invocation, exec engine.Execution) string {
		return containerfilter.SummarizeDockerTransfer(exec.Stdout+"\n"+exec.Stderr, maxLines)
	}
}

func streamRenderDockerTransfer(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(
		true,
		true,
		func(input string) string {
			return containerfilter.SummarizeDockerTransfer(input, budget.MaxLines)
		},
		func(input string) (string, string, bool) {
			return containerfilter.DockerTransferRecoveryInfo(input, budget.MaxLines)
		},
	)
}

func dockerTransferProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "docker-transfer",
		Description:      "Collapses per-layer `docker pull`/`docker push` progress into layer counts, digests, and final status.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match:            matchDockerTransfer,
		Render:           renderDockerTransfer(maxLines),
		StreamRender:     streamRenderDockerTransfer,
		ParseBytes:       profilekit.ParseCombined,
		Explain:          dockerTransferExplain,
	}
}

var dockerActivityExplain = []string{
	"Suppresses per-service progress spinners, pull chatter, and BuildKit step noise from compose activity.",
	"Keeps `Container x Started/Healthy` state lines, build step failures, error blocks, and error-bearing attach output.",
}

func matchDockerActivity(inv engine.Invocation) bool {
	return isDockerComposeCommand(inv.Display, "up") ||
		isDockerComposeCommand(inv.Display, "build") ||
		isDockerComposeCommand(inv.Display, "down") ||
		profilekit.HasCommand(inv.Display, "docker", "run")
}

func renderDockerActivity(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(_ engine.Invocation, exec engine.Execution) string {
		return containerfilter.SummarizeComposeActivity(exec.Stdout+"\n"+exec.Stderr, maxLines)
	}
}

func streamRenderDockerActivity(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(
		true,
		true,
		func(input string) string {
			return containerfilter.SummarizeComposeActivity(input, budget.MaxLines)
		},
		func(input string) (string, string, bool) {
			return containerfilter.ComposeActivityRecoveryInfo(input, budget.MaxLines)
		},
	)
}

func dockerActivityProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "docker-activity",
		Description:      "Keeps service state transitions and failures from `docker compose up/build/down` and `docker run` while dropping progress noise.",
		Confidence:       engine.ConfidenceMedium,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match:            matchDockerActivity,
		Render:           renderDockerActivity(maxLines),
		StreamRender:     streamRenderDockerActivity,
		ParseBytes:       profilekit.ParseCombined,
		Explain:          dockerActivityExplain,
	}
}

func dockerPSProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "docker-ps",
		Description:      "Summarizes container state for `docker ps` and `docker compose ps`.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match: func(inv engine.Invocation) bool {
			return profilekit.HasCommand(inv.Display, "docker", "ps") || isDockerComposeCommand(inv.Display, "ps")
		},
		Prepare: func(inv engine.Invocation) []string {
			return prepareDockerPSCommand(inv.Command)
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return containerfilter.SummarizeDockerPS(exec.Stdout, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(
				true,
				false,
				func(input string) string {
					return containerfilter.SummarizeDockerPS(input, budget.MaxLines)
				},
				func(input string) (string, string, bool) {
					return containerfilter.DockerPSRecoveryInfo(input, budget.MaxLines)
				},
			)
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Uses compact state-oriented output for `docker ps` and JSON for `docker compose ps` when the user did not already request a format.",
			"Highlights running versus exited containers and keeps service or image identifiers visible.",
		},
	}
}

func dockerLogsProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "docker-logs",
		Description:      "Collapses repetitive container logs and preserves error-bearing lines by service or container.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match: func(inv engine.Invocation) bool {
			return profilekit.HasCommand(inv.Display, "docker", "logs") || isDockerComposeCommand(inv.Display, "logs")
		},
		Prepare: func(inv engine.Invocation) []string {
			return prepareDockerLogsCommand(inv.Command)
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return containerfilter.SummarizeDockerLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(
				true,
				true,
				func(input string) string {
					return containerfilter.SummarizeDockerLogs(input, budget.MaxLines)
				},
				func(input string) (string, string, bool) {
					return containerfilter.DockerLogsRecoveryInfo(input, budget.MaxLines)
				},
			)
		},
		ParseBytes: profilekit.ParseCombined,
		Explain: []string{
			"Normalizes `docker logs` and `docker compose logs` toward bounded tails when the user did not already choose a window.",
			"Groups repeated failures by service or container and keeps the first error-bearing lines instead of raw repetition.",
		},
	}
}

func prepareDockerPSCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	switch {
	case profilekit.HasCommand(command, "docker", "ps"):
		if profilekit.ContainsAny(command[2:], "--format") || profilekit.ContainsPrefix(command[2:], "--format=") {
			return command
		}
		return append(command, "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}")
	case isDockerComposeCommand(command, "ps"):
		if profilekit.ContainsAny(command[3:], "--format") || profilekit.ContainsPrefix(command[3:], "--format=") {
			return command
		}
		return append(command, "--format", "json")
	default:
		return command
	}
}

func prepareDockerLogsCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}
	switch {
	case profilekit.HasCommand(command, "docker", "logs"):
		if hasDockerLogWindow(command[2:]) {
			return command
		}
		return insertAfterDockerSubcommand(command, "--tail", "200")
	case isDockerComposeCommand(command, "logs"):
		if hasDockerLogWindow(command[3:]) {
			return command
		}
		return insertAfterDockerSubcommand(command, "--tail", "200")
	default:
		return command
	}
}

func hasDockerLogWindow(args []string) bool {
	return profilekit.ContainsAny(args, "--tail") ||
		profilekit.ContainsPrefix(args, "--tail=") ||
		profilekit.ContainsAny(args, "--since", "-f", "--follow")
}
