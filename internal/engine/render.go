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
	emptyResult  bool
	recoveryPlan RecoveryPlan
}

type RenderedExecution struct {
	Text            string
	RawCombined     string
	BytesParsed     int
	BytesEmitted    int
	RawTokens       int
	FilteredTokens  int
	FallbackUsed    bool
	EmptyResult     bool
	VerifierRepairs int
}

func renderProfile(profile Profile, inv Invocation, exec Execution, fallbackLines int, passthrough bool) renderResult {
	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	if passthrough {
		return renderResult{
			text:        rawCombined,
			bytesParsed: len(rawCombined),
		}
	}

	budget := ResolveBudget(profile, inv, fallbackLines)
	if profile.StreamRender != nil {
		reducer := profile.StreamRender(inv, budget)
		if reducer != nil {
			return renderReducerProfile(profile, exec, reducer, budget, rawCombined)
		}
	}

	rendered := rawCombined
	if profile.Render != nil {
		rendered = profile.Render(inv, exec)
	}
	fallbackUsed := false
	emptyResult := false
	if strings.TrimSpace(rendered) == "" {
		rendered = renderPreferredStreamFallback(profile, exec, budget)
	}
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
		fallbackUsed = true
		emptyResult = true
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
		emptyResult:  emptyResult,
	}
}

func renderReducerProfile(profile Profile, exec Execution, reducer StreamReducer, budget OutputBudget, rawCombined string) renderResult {
	feedReducer(profile.StreamPreference, reducer, exec)
	text := reducer.Result()
	emptyResult := false
	if strings.TrimSpace(text) == "" {
		text = renderPreferredStreamFallback(profile, exec, budget)
	}
	if strings.TrimSpace(text) == "" {
		text = rawCombined
		emptyResult = true
	}
	return renderResult{
		text:         text,
		bytesParsed:  maxInt(reducer.BytesParsed(), 0),
		fallbackUsed: reducer.FallbackUsed() || profile.Name == "passthrough",
		emptyResult:  emptyResult,
		recoveryPlan: reducerRecoveryPlan(reducer),
	}
}

func RenderExecution(profile Profile, inv Invocation, exec Execution, fallbackLines int, passthrough bool) RenderedExecution {
	rawCombined := combineStreams(exec.Stdout, exec.Stderr)
	memo := history.TokenMemo{}
	rawTokens := memo.Estimate(rawCombined)
	budget := ResolveBudget(profile, inv, fallbackLines)
	rendered := renderProfile(profile, inv, exec, fallbackLines, passthrough)
	text := rendered.text
	if shouldUseFailureEscape(profile, exec.ExitCode, passthrough, rendered.fallbackUsed) && rawCombined != "" {
		escapeBudget := ExpandBudgetForFailureEscape(budget, inv)
		if escaped := filters.CompactLines(rawCombined, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
			text = escaped
		}
	}
	text = applyUltraCompactRender(inv, exec, text, rawCombined)
	text, _, _ = enforceCompressionContract(text, rawCombined, rawTokens, budget, rendered.recoveryPlan, passthrough, inv.Advanced.CompressionContract, memo)
	text = ensureInformativeFailureRender(streamingRenderInput{
		profile:     profile,
		inv:         inv,
		exec:        exec,
		budget:      budget,
		raw:         rawCombined,
		passthrough: passthrough,
	}, text)
	if shouldGuardSmallOutput(profile, passthrough) && !inv.UltraCompact {
		text = preferRawSmallOutputForProfile(profile, text, rawCombined, exec.ExitCode)
	}
	text, verification := applyRetentionVerifier(retentionVerifyInput{
		rendered:    text,
		rawCombined: rawCombined,
		rawTokens:   rawTokens,
		exitCode:    exec.ExitCode,
		profile:     profile,
		budget:      budget,
		inv:         inv,
		passthrough: passthrough,
		memo:        memo,
	})
	// Batch executions always hold the complete streams, so the final
	// never-worse-than-raw invariant applies unconditionally.
	text = enforceFinalNeverWorseThanRaw(text, rawCombined, passthrough, true, inv.UltraCompact, memo)
	return RenderedExecution{
		Text:            text,
		RawCombined:     rawCombined,
		BytesParsed:     rendered.bytesParsed,
		BytesEmitted:    len(text),
		RawTokens:       rawTokens,
		FilteredTokens:  memo.Estimate(text),
		FallbackUsed:    rendered.fallbackUsed,
		EmptyResult:     rendered.emptyResult,
		VerifierRepairs: verification.repairs,
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
