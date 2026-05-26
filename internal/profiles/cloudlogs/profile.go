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
