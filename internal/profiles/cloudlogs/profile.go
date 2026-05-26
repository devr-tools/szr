package cloudlogs

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlogs"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "supabase-function-logs",
			Description:      "Targets Supabase Edge Function logs with scoped JSON rewrites.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isSupabaseFunctionLogsCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareSupabaseFunctionLogsCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cloudfilter.SummarizeCloudLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return cloudfilter.SummarizeCloudLogs(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `supabase functions logs` before the broader cloud log profile.",
				"Requests JSON output so function slug, path, status code, and repeated invocation failures can be grouped reliably.",
			},
		},
		{
			Name:             "heroku-router-logs",
			Description:      "Biases Heroku log views toward router failures and request-path signal.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isHerokuRouterLogsCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareHerokuRouterLogsCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cloudfilter.SummarizeCloudLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return cloudfilter.SummarizeCloudLogs(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `heroku logs` before the broader cloud log profile when the user did not already scope by dyno or source.",
				"Adds `--source heroku` so router codes, status, dyno, and request paths dominate the preview instead of app noise.",
			},
		},
		{
			Name:             "cloud-logs",
			Description:      "Summarizes common cloud log streams around time ranges, sources, and repeated error signatures.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isCloudLogsCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareCloudLogsCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cloudfilter.SummarizeCloudLogs(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return cloudfilter.SummarizeCloudLogs(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `aws logs`, `gcloud logging read`, and `az monitor` log retrieval commands.",
				"Requests structured output when the CLI supports it and groups repeated failures by source instead of replaying full event streams.",
			},
		},
	}
}
