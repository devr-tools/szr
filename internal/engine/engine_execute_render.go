package engine

import (
	"io"
	"os"
	"strings"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
)

func renderStreamingOutput(
	profile Profile,
	preparedInv Invocation,
	execResult Execution,
	streamReducer StreamReducer,
	budget OutputBudget,
	rawCombined string,
	rawTokens int,
	passthrough bool,
	fastPath FastPathDecision,
	rawBytesRead int,
	captureTruncated bool,
	commandRewritten bool,
	failureArtifactPath string,
	memo history.TokenMemo,
) (string, bool, bool, RecoveryPlan) {
	rendered := rawCombined
	if !passthrough {
		rendered = renderedStreamingContent(profile, preparedInv, execResult, streamReducer, rawCombined, fastPath, rawBytesRead, captureTruncated)
		if strings.TrimSpace(rendered) == "" {
			rendered = renderPreferredStreamFallback(profile, execResult, budget)
		}
	}
	fallbackUsed, emptyResult := streamingFallbackUsed(profile, streamReducer, passthrough, rendered, rawCombined)
	recoveryPlan := reducerRecoveryPlan(streamReducer)
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
	}
	if shouldUseFailureEscape(profile, execResult.ExitCode, passthrough, fallbackUsed) && rawCombined != "" {
		escapeBudget := ExpandBudgetForFailureEscape(budget, preparedInv)
		if escaped := filters.CompactLines(rawCombined, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
			rendered = escaped
		}
	}
	rendered = applyUltraCompactRender(preparedInv, execResult, rendered, rawCombined)
	rendered, recoveryPlan, _ = enforceCompressionContract(rendered, rawCombined, rawTokens, budget, recoveryPlan, passthrough, preparedInv.Advanced.CompressionContract, memo)
	rendered = ensureInformativeFailureRender(profile, preparedInv, rendered, rawCombined, failureArtifactPath, execResult.ExitCode, passthrough, budget)
	rendered = preferTerseRenderForRewrittenCommand(rendered, rawCombined, execResult.ExitCode, commandRewritten, passthrough, captureTruncated, memo)
	if !captureTruncated && shouldGuardSmallOutput(profile, passthrough) && !preparedInv.UltraCompact {
		rendered = preferRawSmallOutputForProfile(profile, rendered, rawCombined, execResult.ExitCode)
	}
	return rendered, fallbackUsed, emptyResult, recoveryPlan
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

// executionBaselineCommand is the argv the user's command would have
// executed without any profile Prepare rewrite: the original wrapper argv
// for unwrapped shell invocations, the prepared command otherwise.
func executionBaselineCommand(inv Invocation) []string {
	if inv.ShellWrap != nil {
		return inv.ShellWrap.Original
	}
	return inv.Command
}

func commandWasRewritten(original []string, prepared []string) bool {
	if len(original) != len(prepared) {
		return true
	}
	for i := range original {
		if original[i] != prepared[i] {
			return true
		}
	}
	return false
}

// ensureInformativeFailureRender guarantees a failing command never renders
// content-free output (only ellipsis markers or artifact bookkeeping). The
// failure escape earlier in the pipeline can be undone by the compression
// contract or an over-aggressive reducer; when that happens, compact raw
// lines within the failure-escape budget are strictly more useful than a
// marker that spends tokens on zero signal. Fidelity beats savings on
// failures, but never past the raw output itself: the final
// never-worse-than-raw guard still caps the finished display at raw cost.
func ensureInformativeFailureRender(
	profile Profile,
	inv Invocation,
	rendered string,
	rawCombined string,
	artifactPath string,
	exitCode int,
	passthrough bool,
	budget OutputBudget,
) string {
	if passthrough || !isFailureExit(profile, exitCode) || !isContentFreeRender(rendered) {
		return rendered
	}
	raw := failureEscapeSource(rawCombined, artifactPath)
	if raw == "" {
		return rendered
	}
	escapeBudget := ExpandBudgetForFailureEscape(budget, inv)
	if escaped := filters.CompactLines(raw, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
		return escaped
	}
	return rendered
}

// failureEscapeSource returns the raw text the failure escape should compact:
// the in-memory capture when present, otherwise the tee artifact.
func failureEscapeSource(rawCombined string, artifactPath string) string {
	if strings.TrimSpace(rawCombined) != "" {
		return rawCombined
	}
	if artifact := readFailureArtifact(artifactPath); strings.TrimSpace(artifact) != "" {
		return artifact
	}
	return ""
}

const failureArtifactReadLimit = 256 * 1024

// readFailureArtifact recovers raw output for the failure escape when the
// in-memory capture is empty. Stream profiles that reduce only one stream
// (for example kubectl-get, which is stdout-only) buffer only a bounded
// preview of the other stream — yet on failure that other stream usually
// carries the diagnostics. The tee artifact written during the run holds the
// full interleaved output; a bounded prefix is enough for a compact escape
// render.
func readFailureArtifact(path string) string {
	if path == "" {
		return ""
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = file.Close() }()
	data := make([]byte, failureArtifactReadLimit)
	n, _ := io.ReadFull(file, data)
	return string(data[:n])
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
	compact := strings.TrimSpace(filters.CompactLines(rawCombined, rewrittenCommandTerseRenderMaxLines))
	if compact == "" {
		return rendered
	}
	// A compact view that had to omit lines is a lossy head-chop, not a
	// faithful terse summary; it may never replace a real render. And a
	// marginal size win does not justify discarding a structured render —
	// only a drastic one (the "plain output would have been one line" case).
	if strings.Contains(compact, "... +") {
		return rendered
	}
	if history.EstimateTokens(compact)*3 >= renderedTokens {
		return rendered
	}
	return compact
}

func renderedStreamingContent(
	profile Profile,
	preparedInv Invocation,
	execResult Execution,
	streamReducer StreamReducer,
	rawCombined string,
	fastPath FastPathDecision,
	rawBytesRead int,
	captureTruncated bool,
) string {
	switch {
	case shouldApplyBypass(profile, fastPath):
		if summary, ok := reducerSummaryForBypass(execResult.ExitCode, streamReducer, rawCombined, rawBytesRead, captureTruncated); ok {
			return summary
		}
		return bypassRawContent(profile, execResult, rawCombined)
	case streamReducer != nil:
		return streamReducer.Result()
	case profile.Render != nil:
		return profile.Render(preparedInv, execResult)
	default:
		return rawCombined
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
