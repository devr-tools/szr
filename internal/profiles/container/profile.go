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
		dockerLogsProfile(maxLines),
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
			return shared.NewBufferedTextReducer(true, false, func(input string) string {
				return containerfilter.SummarizeDockerPS(input, budget.MaxLines)
			})
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
			return shared.NewBufferedTextReducer(true, true, func(input string) string {
				return containerfilter.SummarizeDockerLogs(input, budget.MaxLines)
			})
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
