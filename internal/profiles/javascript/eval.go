package javascript

import (
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

// nodeEvalProfile routes inline `node -e`/`-p` scripts through the generic
// failure reducer. Node writes thrown errors and stack traces to stderr, so
// stderr leads, and the budget floors keep at least one failure line and one
// stack anchor (including node's `[eval]:1:7` pseudo-file frames) in view.
//
//nolint:maintidx // Profile constructors are declarative and intentionally keep match/render behavior together.
func nodeEvalProfile(maxLines int) engine.Profile {
	lines := profilekit.AtLeast(maxLines, 10)
	return engine.Profile{
		Name:        "node-eval",
		Description: "Summarizes inline `node -e`/`-p` script runs around errors and stack anchors.",
		Confidence:  engine.ConfidenceMedium,
		Capabilities: engine.ProfileCapabilities{
			FastPathBypass:     engine.FastPathBypassSmallOutput,
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStderrFirst,
		Budget:           nodeEvalBudget(lines),
		LatencyBudget:    profilekit.LatencyBudget(25),
		Match:            matchNodeEval,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return renderNodeEval(exec, lines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return nodeEvalReducer(budget)
		},
		ParseBytes: profilekit.ParseStderrFirst,
		Explain: []string{
			"Matches `node -e`, `--eval`, `-p`, and `--print` inline scripts, directly or behind a shell `-c` wrapper.",
			"Failure runs keep the thrown error and its `[eval]`/file stack anchors; passing runs render as compact lines.",
		},
	}
}

func matchNodeEval(inv engine.Invocation) bool {
	return inv.Classification.Command.JavaScript.IsNodeEval || inv.Classification.Display.JavaScript.IsNodeEval
}

func renderNodeEval(exec engine.Execution, lines int) string {
	if exec.ExitCode != 0 {
		return filters.SummarizeGenericFailure(exec.Stderr+"\n"+exec.Stdout, lines)
	}
	return filters.CompactLines(exec.Stdout+"\n"+exec.Stderr, lines)
}

func nodeEvalBudget(lines int) engine.OutputBudget {
	return engine.OutputBudget{
		MaxLines:    lines,
		MaxBytes:    lines * 160,
		MaxTokens:   lines * 32,
		MinFailures: 1,
		MinAnchors:  1,
		MinHints:    1,
	}
}

func nodeEvalReducer(budget engine.OutputBudget) engine.StreamReducer {
	return filters.NewGenericFailureReducerWithOptions(filters.GenericFailureReducerOptions{
		MaxLines:           budget.MaxLines,
		MaxBytes:           budget.MaxBytes,
		MinFailures:        budget.MinFailures,
		MinAnchors:         budget.MinAnchors,
		MinHints:           budget.MinHints,
		NoisePrefiltering:  budget.NoisePrefiltering,
		SemanticCompaction: budget.SemanticCompaction,
	})
}
