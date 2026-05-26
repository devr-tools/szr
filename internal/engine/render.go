package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

type renderResult struct {
	text         string
	bytesParsed  int
	fallbackUsed bool
	recoveryPlan RecoveryPlan
}

type RenderedExecution struct {
	Text           string
	RawCombined    string
	BytesParsed    int
	BytesEmitted   int
	RawTokens      int
	FilteredTokens int
	FallbackUsed   bool
}

func renderProfile(profile Profile, inv Invocation, exec Execution, fallbackLines int, passthrough bool) renderResult {
	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	if passthrough {
		return renderResult{
			text:        rawCombined,
			bytesParsed: len(rawCombined),
		}
	}

	if profile.StreamRender != nil {
		budget := ResolveBudget(profile, inv, fallbackLines)
		reducer := profile.StreamRender(inv, budget)
		if reducer != nil {
			feedReducer(profile.StreamPreference, reducer, exec)
			text := reducer.Result()
			if strings.TrimSpace(text) == "" {
				text = rawCombined
			}
			return renderResult{
				text:         text,
				bytesParsed:  maxInt(reducer.BytesParsed(), 0),
				fallbackUsed: reducer.FallbackUsed() || profile.Name == "passthrough",
				recoveryPlan: reducerRecoveryPlan(reducer),
			}
		}
	}

	rendered := rawCombined
	if profile.Render != nil {
		rendered = profile.Render(inv, exec)
	}
	fallbackUsed := false
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
		fallbackUsed = true
	}
	if profile.Name == "passthrough" {
		fallbackUsed = true
	}
	bytesParsed := len(rawCombined)
	if profile.ParseBytes != nil {
		bytesParsed = maxInt(profile.ParseBytes(exec), 0)
	}
	return renderResult{
		text:         rendered,
		bytesParsed:  bytesParsed,
		fallbackUsed: fallbackUsed,
	}
}

func RenderExecution(profile Profile, inv Invocation, exec Execution, fallbackLines int, passthrough bool) RenderedExecution {
	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	budget := ResolveBudget(profile, inv, fallbackLines)
	rendered := renderProfile(profile, inv, exec, fallbackLines, passthrough)
	text := rendered.text
	if shouldUseFailureEscape(profile, exec.ExitCode, passthrough, rendered.fallbackUsed) && rawCombined != "" {
		escapeBudget := ExpandBudgetForFailureEscape(budget, inv)
		if escaped := filters.CompactLines(rawCombined, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
			text = escaped
		}
	}
	text, _, _ = enforceCompressionContract(text, rawCombined, budget, rendered.recoveryPlan, passthrough, inv.Advanced.CompressionContract)
	return RenderedExecution{
		Text:           text,
		RawCombined:    rawCombined,
		BytesParsed:    rendered.bytesParsed,
		BytesEmitted:   len(text),
		RawTokens:      history.EstimateTokens(rawCombined),
		FilteredTokens: history.EstimateTokens(text),
		FallbackUsed:   rendered.fallbackUsed,
	}
}

func feedReducer(preference string, reducer StreamReducer, exec Execution) {
	switch preference {
	case StreamStdoutOnly:
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
	case StreamStderrOnly:
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
	case StreamStderrFirst:
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
	default:
		if exec.Stdout != "" {
			reducer.ConsumeStdout([]byte(exec.Stdout))
		}
		if exec.Stderr != "" {
			reducer.ConsumeStderr([]byte(exec.Stderr))
		}
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
