package profilekit

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
)

type SummaryConfig struct {
	StreamPreference string
	Budget           engine.OutputBudget
	LatencyBudget    int
	Render           func(engine.Invocation, engine.Execution) string
	StreamRender     engine.StreamRenderFactory
	ParseBytes       func(engine.Execution) int
}

func WithSummary(profile engine.Profile, config SummaryConfig) engine.Profile {
	profile.StreamPreference = config.StreamPreference
	profile.Budget = config.Budget
	profile.LatencyBudget = LatencyBudget(config.LatencyBudget)
	profile.Render = config.Render
	profile.StreamRender = config.StreamRender
	profile.ParseBytes = config.ParseBytes
	return profile
}

func CombinedBufferedSummary(maxLines, floor, latencyMS int, preference string, summarize func(string, int) string) SummaryConfig {
	limit := AtLeast(maxLines, floor)
	return SummaryConfig{
		StreamPreference: preference,
		Budget:           OutputBudget(limit),
		LatencyBudget:    latencyMS,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return summarize(exec.Stdout+"\n"+exec.Stderr, limit)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducer(true, true, func(input string) string {
				return summarize(input, budget.MaxLines)
			})
		},
		ParseBytes: ParseCombined,
	}
}

// CombinedBufferedContractSummary is CombinedBufferedSummary for summarize
// functions that strip ANSI themselves and can self-cap to the engine
// compression contract (see filters.PredictedTokenAllowance): the summarize
// function receives the invocation's contract flag, and the streaming
// reducer hands it the unstripped stream so the raw-size prediction matches
// the raw cost the contract budgets against. The batch render path already
// passes the streams unstripped.
func CombinedBufferedContractSummary(maxLines, floor, latencyMS int, preference string, summarize func(string, int, bool) string) SummaryConfig {
	limit := AtLeast(maxLines, floor)
	return SummaryConfig{
		StreamPreference: preference,
		Budget:           OutputBudget(limit),
		LatencyBudget:    latencyMS,
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return summarize(exec.Stdout+"\n"+exec.Stderr, limit, inv.Advanced.CompressionContract)
		},
		StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedRawTextReducer(true, true, func(input string) string {
				return summarize(input, budget.MaxLines, inv.Advanced.CompressionContract)
			}, nil)
		},
		ParseBytes: ParseCombined,
	}
}

func StdoutSummary(maxLines, floor, latencyMS int, preference string, render func(string) string, stream func(engine.OutputBudget) engine.StreamReducer) SummaryConfig {
	limit := AtLeast(maxLines, floor)
	return SummaryConfig{
		StreamPreference: preference,
		Budget:           OutputBudget(limit),
		LatencyBudget:    latencyMS,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return render(exec.Stdout)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return stream(budget)
		},
		ParseBytes: ParseStdout,
	}
}
