package profiles

import (
	"szr/internal/engine"
	"szr/internal/filters"
)

func containerProfiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "docker-ps",
			Description:      "Summarizes container state for `docker ps` and `docker compose ps`.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           outputBudget(atLeast(maxLines, 10)),
			LatencyBudget:    latencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "docker", "ps") || isDockerComposeCommand(inv.Display, "ps")
			},
			Prepare: func(inv engine.Invocation) []string {
				if len(inv.Command) == 0 {
					return inv.Command
				}
				switch {
				case hasCommand(inv.Command, "docker", "ps"):
					if containsAny(inv.Command[2:], "--format") || containsPrefix(inv.Command[2:], "--format=") {
						return inv.Command
					}
					return append(inv.Command, "--format", "{{.Names}}\t{{.Status}}\t{{.Image}}")
				case isDockerComposeCommand(inv.Command, "ps"):
					if containsAny(inv.Command[3:], "--format") || containsPrefix(inv.Command[3:], "--format=") {
						return inv.Command
					}
					return append(inv.Command, "--format", "json")
				default:
					return inv.Command
				}
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeDockerPS(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, false, func(input string) string {
					return filters.SummarizeDockerPS(input, maxLines)
				})
			},
			ParseBytes: parseStdout,
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
			Budget:           outputBudget(atLeast(maxLines, 12)),
			LatencyBudget:    latencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return hasCommand(inv.Display, "docker", "logs") || isDockerComposeCommand(inv.Display, "logs")
			},
			Prepare: func(inv engine.Invocation) []string {
				if len(inv.Command) == 0 {
					return inv.Command
				}
				switch {
				case hasCommand(inv.Command, "docker", "logs"):
					if containsAny(inv.Command[2:], "--tail") || containsPrefix(inv.Command[2:], "--tail=") || containsAny(inv.Command[2:], "--since", "-f", "--follow") {
						return inv.Command
					}
					return insertAfterDockerSubcommand(inv.Command, "--tail", "200")
				case isDockerComposeCommand(inv.Command, "logs"):
					if containsAny(inv.Command[3:], "--tail") || containsPrefix(inv.Command[3:], "--tail=") || containsAny(inv.Command[3:], "--since", "-f", "--follow") {
						return inv.Command
					}
					return insertAfterDockerSubcommand(inv.Command, "--tail", "200")
				default:
					return inv.Command
				}
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeDockerLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return filters.NewBufferedTextReducer(true, true, func(input string) string {
					return filters.SummarizeDockerLogs(input, maxLines)
				})
			},
			ParseBytes: parseCombined,
			Explain: []string{
				"Normalizes `docker logs` and `docker compose logs` toward bounded tails when the user did not already choose a window.",
				"Groups repeated failures by service or container and keeps the first error-bearing lines instead of raw repetition.",
			},
		},
	}
}
