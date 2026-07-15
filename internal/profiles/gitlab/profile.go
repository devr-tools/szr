package gitlab

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	gitlabfilter "github.com/devr-tools/szr/internal/filters/gitlab"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{glabPipelineProfile(maxLines)}
}

var glabPipelineExplain = []string{
	"Matches `glab ci status`, `glab ci list`, and `glab pipeline list` job and pipeline tables.",
	"Leads with a status breakdown, keeps failed, running, and canceled rows in full, and folds the dominant status into a count.",
}

func renderGlabPipeline(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(_ engine.Invocation, exec engine.Execution) string {
		return gitlabfilter.SummarizePipelines(exec.Stdout+"\n"+exec.Stderr, maxLines)
	}
}

func streamRenderGlabPipeline(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(
		true,
		true,
		func(input string) string {
			return gitlabfilter.SummarizePipelines(input, budget.MaxLines)
		},
		func(input string) (string, string, bool) {
			return gitlabfilter.PipelineRecoveryInfo(input, budget.MaxLines)
		},
	)
}

func glabPipelineProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "glab-pipeline",
		Description:      "Summarizes glab pipeline and CI status listings into status counts plus the failed or still-moving rows.",
		Confidence:       engine.ConfidenceMedium,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match:            matchGlabPipeline,
		Render:           renderGlabPipeline(maxLines),
		StreamRender:     streamRenderGlabPipeline,
		ParseBytes:       profilekit.ParseCombined,
		Explain:          glabPipelineExplain,
	}
}

var glabPipelineSubcommands = map[string]map[string]bool{
	"ci":       {"status": true, "list": true, "ls": true},
	"pipeline": {"status": true, "list": true, "ls": true},
}

func matchGlabPipeline(inv engine.Invocation) bool {
	head, sub, ok := glabCommand(inv.Display)
	if !ok {
		return false
	}
	subs, known := glabPipelineSubcommands[head]
	return known && subs[sub]
}

// glabCommand extracts the first two non-flag arguments of a glab
// invocation, skipping value-carrying global flags such as -R.
func glabCommand(args []string) (string, string, bool) {
	if len(args) == 0 || args[0] != "glab" {
		return "", "", false
	}
	rest := make([]string, 0, 2)
	for i := 1; i < len(args) && len(rest) < 2; {
		if strings.HasPrefix(args[i], "-") {
			i = skipGlabOption(args, i)
			continue
		}
		rest = append(rest, args[i])
		i++
	}
	if len(rest) < 2 {
		return "", "", false
	}
	return rest[0], rest[1], true
}

func skipGlabOption(args []string, i int) int {
	if strings.Contains(args[i], "=") {
		return i + 1
	}
	switch args[i] {
	case "-R", "--repo", "--hostname":
		return i + 2
	}
	return i + 1
}
