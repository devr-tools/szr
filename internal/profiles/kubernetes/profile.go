package kubernetes

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	kubefilter "github.com/devr-tools/szr/internal/filters/kubernetes"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "kubectl-get",
			Description:      "Summarizes `kubectl get` results around resource names, namespaces, and live status.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isKubectlCommand(inv.Display, "get")
			},
			Prepare: func(inv engine.Invocation) []string {
				if containsAny(inv.Command, "-o", "--output", "-w", "--watch", "--watch-only") || containsPrefix(inv.Command, "-o=") || containsPrefix(inv.Command, "--output=") {
					return inv.Command
				}
				return append(inv.Command, "-o", "json")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return kubefilter.SummarizeGet(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return kubefilter.SummarizeGet(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Pushes `kubectl get` toward JSON when the user did not already request another output or watch mode.",
				"Groups results by resource with namespace, readiness, replica, and phase signal instead of raw tables.",
			},
		},
		{
			Name:             "kubectl-describe",
			Description:      "Preserves object identity and warning events from `kubectl describe` output.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isKubectlCommand(inv.Display, "describe")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return kubefilter.SummarizeDescribe(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return kubefilter.SummarizeDescribe(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Keeps object metadata such as name, namespace, node, IP, and status.",
				"Pulls warning-bearing events and failure reasons forward so the agent sees the operational cause quickly.",
			},
		},
		{
			Name:             "kubectl-logs",
			Description:      "Bounds `kubectl logs` output and keeps repeated error lines grouped by prefixed source.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isKubectlCommand(inv.Display, "logs")
			},
			Prepare: func(inv engine.Invocation) []string {
				command := inv.Command
				if !containsAny(command, "--prefix") {
					command = insertKubectlVerbArgs(command, "--prefix")
				}
				if containsAny(command, "--tail", "--since", "--since-time", "-f", "--follow") || containsPrefix(command, "--tail=") || containsPrefix(command, "--since=") || containsPrefix(command, "--since-time=") {
					return command
				}
				return insertKubectlVerbArgs(command, "--tail=200")
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return kubefilter.SummarizeLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return kubefilter.SummarizeLogs(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Adds `--prefix` and a bounded tail for plain `kubectl logs` invocations so source identity survives reduction.",
				"Collapses repeated failures by pod or container while leaving error-bearing lines visible.",
			},
		},
		{
			Name:             "kubectl-top",
			Description:      "Keeps the highest-signal `kubectl top` rows visible without a full metrics table.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isKubectlCommand(inv.Display, "top")
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return shared.CompactLines(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return shared.CompactLines(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Matches `kubectl top` to keep the newest metric rows visible without preserving the whole table.",
				"Uses a compact preview reducer because `kubectl top` is already terse but often wider than needed.",
			},
		},
		{
			Name:             "kubectl-events",
			Description:      "Surfaces warning-bearing `kubectl events` lines ahead of long event streams.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isKubectlCommand(inv.Display, "events")
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return kubefilter.SummarizeDescribe(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return kubefilter.SummarizeDescribe(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Matches `kubectl events` so warning-bearing event lines are easy to spot.",
				"Reuses the describe/event summarizer to keep failure reasons and backoff-style events near the top.",
			},
		},
	}
}
