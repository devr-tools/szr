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
			Name:             "vercel-deployments",
			Description:      "Targets Vercel deployment listings with deployment-specific JSON rewrites and summaries.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "vercel" && isVercelDeploymentCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareVercelDeploymentCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return cloudfilter.SummarizeCloudList(exec.Stdout, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					false,
					func(input string) string {
						return cloudfilter.SummarizeCloudList(input, budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return cloudfilter.CloudListRecoveryInfo(input, budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Matches `vercel list` and `vercel ls` before the broader cloud inventory profile.",
				"Requests JSON plus metadata so deployment target, state, URL, and branch context survive reduction.",
			},
		},
		{
			Name:             "cloud-list",
			Description:      "Summarizes common cloud inventory commands around resource identity and state.",
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
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					false,
					func(input string) string {
						return cloudfilter.SummarizeCloudList(input, budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return cloudfilter.CloudListRecoveryInfo(input, budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Matches inventory-style cloud and platform CLI commands such as `aws`, `gcloud`, `az`, `doctl`, `oci`, `openstack`, `vercel`, `supabase`, and `heroku`.",
				"Requests structured JSON output when the user did not already choose another formatter, then reduces it to resource identity and state.",
			},
		},
	}
}
