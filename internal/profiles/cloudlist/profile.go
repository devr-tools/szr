package cloudlist

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	cloudfilter "github.com/devr-tools/szr/internal/filters/cloudlist"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "cloud-list",
			Description:      "Summarizes common cloud inventory commands for AWS, Google Cloud, and Azure around resource identity and state.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isCloudListCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareCloudListCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cloudfilter.SummarizeCloudList(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, false, func(input string) string {
					return cloudfilter.SummarizeCloudList(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Matches inventory-style `aws`, `gcloud`, and `az` commands that list, describe, show, or get cloud resources.",
				"Requests structured JSON output when the user did not already choose another formatter, then reduces it to resource identity and state.",
			},
		},
	}
}
