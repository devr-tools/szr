package container

import (
	"szr/internal/engine"
	shared "szr/internal/filters"
	containerfilter "szr/internal/filters/container"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
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
				if len(inv.Command) == 0 {
					return inv.Command
				}
				switch {
				case profilekit.HasCommand(inv.Command, "docker", "ps"):
					if profilekit.ContainsAny(inv.Command[2:], "--format") || profilekit.ContainsPrefix(inv.Command[2:], "--format=") {
						return inv.Command
					}
					return append(inv.Command, "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}")
				case isDockerComposeCommand(inv.Command, "ps"):
					if profilekit.ContainsAny(inv.Command[3:], "--format") || profilekit.ContainsPrefix(inv.Command[3:], "--format=") {
						return inv.Command
					}
					return append(inv.Command, "--format", "json")
				default:
					return inv.Command
				}
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return containerfilter.SummarizeDockerPS(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return containerfilter.SummarizeDockerPS(input, maxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Uses compact state-oriented output for `docker ps` and JSON for `docker compose ps` when the user did not already request a format.",
				"Highlights running versus exited containers and keeps service or image identifiers visible.",
			},
		},
		{
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
				if len(inv.Command) == 0 {
					return inv.Command
				}
				switch {
				case profilekit.HasCommand(inv.Command, "docker", "logs"):
					if profilekit.ContainsAny(inv.Command[2:], "--tail") || profilekit.ContainsPrefix(inv.Command[2:], "--tail=") || profilekit.ContainsAny(inv.Command[2:], "--since", "-f", "--follow") {
						return inv.Command
					}
					return insertAfterDockerSubcommand(inv.Command, "--tail", "200")
				case isDockerComposeCommand(inv.Command, "logs"):
					if profilekit.ContainsAny(inv.Command[3:], "--tail") || profilekit.ContainsPrefix(inv.Command[3:], "--tail=") || profilekit.ContainsAny(inv.Command[3:], "--since", "-f", "--follow") {
						return inv.Command
					}
					return insertAfterDockerSubcommand(inv.Command, "--tail", "200")
				default:
					return inv.Command
				}
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return containerfilter.SummarizeDockerLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return containerfilter.SummarizeDockerLogs(input, maxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Normalizes `docker logs` and `docker compose logs` toward bounded tails when the user did not already choose a window.",
				"Groups repeated failures by service or container and keeps the first error-bearing lines instead of raw repetition.",
			},
		},
	}
}
