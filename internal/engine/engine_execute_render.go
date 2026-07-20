package engine

import (
	"strings"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

type streamingRenderInput struct {
	profile          Profile
	inv              Invocation
	exec             Execution
	reducer          StreamReducer
	budget           OutputBudget
	raw              string
	rawTokens        int
	passthrough      bool
	fastPath         FastPathDecision
	rawBytes         int
	captureTruncated bool
	commandRewritten bool
	artifactPath     string
	memo             history.TokenMemo
}

func renderStreamingOutput(input streamingRenderInput) (string, bool, bool, RecoveryPlan) {
	rendered := initialStreamingRender(input)
	rendered, fallbackUsed, emptyResult, recoveryPlan := escalateStreamingRender(input, rendered)
	rendered = applyUltraCompactRender(input.inv, input.exec, rendered, input.raw)
	rendered, recoveryPlan, _ = enforceCompressionContract(rendered, input.raw, input.rawTokens, input.budget, recoveryPlan, input.passthrough, input.inv.Advanced.CompressionContract, input.memo)
	rendered = finalizeStreamingRender(input, rendered)
	return rendered, fallbackUsed, emptyResult, recoveryPlan
}

// escalateStreamingRender records the render-health flags for the run, then
// escalates empty renders to raw output and failing renders to the failure
// escape.
func escalateStreamingRender(input streamingRenderInput, rendered string) (string, bool, bool, RecoveryPlan) {
	fallbackUsed, emptyResult := streamingFallbackUsed(input.profile, input.reducer, input.passthrough, rendered, input.raw)
	recoveryPlan := reducerRecoveryPlan(input.reducer)
	if strings.TrimSpace(rendered) == "" {
		rendered = input.raw
	}
	rendered = applyStreamingFailureEscape(input, rendered, fallbackUsed)
	return rendered, fallbackUsed, emptyResult, recoveryPlan
}

// initialStreamingRender produces the first-pass render: the reducer or
// profile render for filtered runs, raw output for passthrough.
func initialStreamingRender(input streamingRenderInput) string {
	if input.passthrough {
		return input.raw
	}
	rendered := renderedStreamingContent(input)
	if strings.TrimSpace(rendered) == "" {
		rendered = renderPreferredStreamFallback(input.profile, input.exec, input.budget)
	}
	return rendered
}

// applyStreamingFailureEscape swaps in compact raw lines when a failing run
// qualifies for the failure escape and the raw capture has content.
func applyStreamingFailureEscape(input streamingRenderInput, rendered string, fallbackUsed bool) string {
	if !shouldUseFailureEscape(input.profile, input.exec.ExitCode, input.passthrough, fallbackUsed) || input.raw == "" {
		return rendered
	}
	escapeBudget := ExpandBudgetForFailureEscape(input.budget, input.inv)
	if escaped := filters.CompactLines(input.raw, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
		return escaped
	}
	return rendered
}

// finalizeStreamingRender applies the trailing render guards: informative
// failure output, terse renders for rewritten commands, and the small-output
// guard for guarded profiles.
func finalizeStreamingRender(input streamingRenderInput, rendered string) string {
	rendered = ensureInformativeFailureRender(input, rendered)
	rendered = preferTerseRenderForRewrittenCommand(rendered, input.raw, input.exec.ExitCode, input.commandRewritten, input.passthrough, input.captureTruncated, input.memo)
	if !input.captureTruncated && shouldGuardSmallOutput(input.profile, input.passthrough) && !input.inv.UltraCompact {
		rendered = preferRawSmallOutputForProfile(input.profile, rendered, input.raw, input.exec.ExitCode)
	}
	return rendered
}

// renderPreferredStreamFallback substitutes a compact view of the unreduced
// stream when a stream-preference profile rendered nothing because the
// command wrote its message to the other stream (kubectl writes "No
// resources found ..." to stderr while the stdout-only reducer sees an empty
// stdout). A compact real message beats an empty render escalated to a raw
// fallback.
func renderPreferredStreamFallback(profile Profile, exec Execution, budget OutputBudget) string {
	var source string
	switch profile.StreamPreference {
	case StreamStdoutOnly:
		if strings.TrimSpace(exec.Stdout) != "" {
			return ""
		}
		source = exec.Stderr
	case StreamStderrOnly:
		if strings.TrimSpace(exec.Stderr) != "" {
			return ""
		}
		source = exec.Stdout
	default:
		return ""
	}
	if strings.TrimSpace(source) == "" {
		return ""
	}
	return strings.TrimSpace(filters.CompactLines(source, budget.MaxLines))
}

const (
	rewrittenCommandTerseRenderMaxTokens = 48
	rewrittenCommandTerseRenderMaxLines  = 12
)

// preferTerseRenderForRewrittenCommand guards renders of commands the
// profile rewrote before execution (for example `go test` -> `go test
// -json`). The generic never-worse-than-raw guard compares the render
// against the REWRITTEN command's raw output; for machine formats that raw
// is enormous, so the guard can never protect what the user's original
// command would have printed (often a tiny `ok <pkg>` line). Passing runs
// therefore get held to a terse standard: once a successful render grows
// past a small constant, fall back to the compact-lines view of the raw
// output when that view is strictly terser. Failures are exempt — there,
// diagnostic fidelity wins over terseness.
func preferTerseRenderForRewrittenCommand(
	rendered string,
	rawCombined string,
	exitCode int,
	commandRewritten bool,
	passthrough bool,
	captureTruncated bool,
	memo history.TokenMemo,
) string {
	// When capture stopped at the preview limit, rawCombined is only the
	// head of the stream; compacting it would replace a correct summary
	// with the least informative part of the output.
	if passthrough || !commandRewritten || exitCode != 0 || captureTruncated {
		return rendered
	}
	renderedTokens := memo.Estimate(rendered)
	if renderedTokens <= rewrittenCommandTerseRenderMaxTokens {
		return rendered
	}
	if compact, ok := terseCompactAlternative(rawCombined, renderedTokens); ok {
		return compact
	}
	return rendered
}

// terseCompactAlternative returns the compact-lines view of the raw output
// when it can stand in for an oversized render of a rewritten command. A
// compact view that had to omit lines is a lossy head-chop, not a faithful
// terse summary; it may never replace a real render. And a marginal size win
// does not justify discarding a structured render — only a drastic one (the
// "plain output would have been one line" case).
func terseCompactAlternative(rawCombined string, renderedTokens int) (string, bool) {
	compact := strings.TrimSpace(filters.CompactLines(rawCombined, rewrittenCommandTerseRenderMaxLines))
	if compact == "" {
		return "", false
	}
	if strings.Contains(compact, "... +") {
		return "", false
	}
	if history.EstimateTokens(compact)*3 >= renderedTokens {
		return "", false
	}
	return compact, true
}

func renderedStreamingContent(input streamingRenderInput) string {
	switch {
	case shouldApplyBypass(input.profile, input.fastPath):
		if summary, ok := reducerSummaryForBypass(input.exec.ExitCode, input.reducer, input.raw, input.rawBytes, input.captureTruncated); ok {
			return summary
		}
		return bypassRawContent(input.profile, input.exec, input.raw)
	case input.reducer != nil:
		return input.reducer.Result()
	case input.profile.Render != nil:
		return input.profile.Render(input.inv, input.exec)
	default:
		return input.raw
	}
}

// bypassRawContent is the raw emission for the tiny-output fast path. The
// bypass decision measured only the profile's preferred stream, and the
// other stream keeps a bounded preview purely for empty-render fallbacks —
// relaying that preview alongside a tiny preferred stream would leak noise
// the profile deliberately ignores.
func bypassRawContent(profile Profile, exec Execution, rawCombined string) string {
	switch profile.StreamPreference {
	case StreamStdoutOnly:
		if strings.TrimSpace(exec.Stdout) != "" {
			return exec.Stdout
		}
	case StreamStderrOnly:
		if strings.TrimSpace(exec.Stderr) != "" {
			return exec.Stderr
		}
	}
	return rawCombined
}

// reducerSummaryForBypass returns the stream reducer's summary when the
// tiny-output fast path fired but the reducer produced a strictly cheaper,
// fully-parsed summary of the same bytes. It is deliberately conservative:
// any hint that the reducer saw less than the full raw output (truncated
// capture buffers from early capture stop or preview limits, fallback mode,
// or a partial parse) keeps the raw bypass. The small-output guard
// (preferRawSmallOutputForProfile) still runs afterwards for guarded
// profiles and may flip back to raw, subject to its own canonical-summary
// exception.
func reducerSummaryForBypass(
	exitCode int,
	streamReducer StreamReducer,
	rawCombined string,
	rawBytesRead int,
	captureTruncated bool,
) (string, bool) {
	if exitCode != 0 || streamReducer == nil || captureTruncated {
		return "", false
	}
	if streamReducer.FallbackUsed() || streamReducer.BytesParsed() < rawBytesRead {
		return "", false
	}
	summary := streamReducer.Result()
	if strings.TrimSpace(summary) == "" {
		return "", false
	}
	if history.EstimateTokens(summary) >= history.EstimateTokens(rawCombined) {
		return "", false
	}
	return summary, true
}

// streamingFallbackUsed reports the render-health flags for one run: whether
// the render fell back to raw output, and whether the profile produced an
// empty render at all (an "empty result"). The two are recorded separately
// so telemetry can tell "the reducer could not parse this" apart from "the
// command produced nothing to render".
func streamingFallbackUsed(profile Profile, streamReducer StreamReducer, passthrough bool, rendered string, rawCombined string) (bool, bool) {
	fallbackUsed := streamReducer != nil && streamReducer.FallbackUsed()
	emptyResult := false
	if strings.TrimSpace(rendered) == "" {
		fallbackUsed = !passthrough || fallbackUsed
		emptyResult = !passthrough
	}
	if !passthrough && profile.Name == "passthrough" {
		fallbackUsed = true
	}
	return fallbackUsed, emptyResult
}
