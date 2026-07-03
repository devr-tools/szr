package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/teeindex"
)

func (e *Engine) Execute(ctx context.Context, inv Invocation, passthrough bool) (Result, error) {
	return e.ExecuteStreaming(ctx, inv, passthrough, nil)
}

func (e *Engine) ExecuteStreaming(
	ctx context.Context,
	inv Invocation,
	passthrough bool,
	onPartial func(PartialResult),
) (Result, error) {
	if len(inv.Command) == 0 {
		return Result{}, fmt.Errorf("missing command")
	}

	preparedInv, profile, command, budget, streamReducer, options, profileConfidence := e.prepareStreamingExecution(inv, passthrough, onPartial)
	runResult, execResult, fastPath, rawCombined, rawBytesRead, rawTokens, duration, err := e.runStreamingCommand(ctx, inv, command, profile, streamReducer, options)
	commandRewritten := commandWasRewritten(preparedInv.Command, command)
	rendered, fallbackUsed, recoveryPlan := renderStreamingOutput(profile, preparedInv, execResult, streamReducer, budget, rawCombined, rawTokens, passthrough, fastPath, rawBytesRead, runResult.captureTruncated, commandRewritten, runResult.teePath)
	teePath := e.ensureStreamingArtifactPath(runResult.teePath, execResult.ExitCode, rawCombined, command, profile, fallbackUsed, recoveryPlan, passthrough)
	rendered = renderedDisplayFinalizer{
		profile:             profile,
		exitCode:            execResult.ExitCode,
		rendered:            rendered,
		rawCombined:         rawCombined,
		rawTokens:           rawTokens,
		budget:              budget,
		plan:                recoveryPlan,
		artifactPath:        teePath,
		passthrough:         passthrough,
		compactArtifactRefs: preparedInv.Advanced.CompactArtifactRefs,
		compressionContract: preparedInv.Advanced.CompressionContract,
		guardSmallOutput:    shouldGuardSmallOutput(profile, passthrough) && !runResult.captureTruncated,
		ultraCompact:        preparedInv.UltraCompact,
	}.finalize()
	rendered, verification := applyRetentionVerifier(retentionVerifyInput{
		rendered:          rendered,
		rawCombined:       rawCombined,
		rawTokens:         rawTokens,
		artifactPath:      teePath,
		captureIncomplete: streamingCaptureIncomplete(runResult),
		exitCode:          execResult.ExitCode,
		profile:           profile,
		budget:            budget,
		inv:               preparedInv,
		passthrough:       passthrough,
		fastPathBypass:    fastPath.BypassCompression,
	})
	bytesParsed := streamingBytesParsed(streamReducer, profile, execResult, rawBytesRead)
	bytesEmitted := len(rendered)
	record := buildStreamingHistoryRecord(inv, profile, profileConfidence, duration, execResult.ExitCode, rawBytesRead, bytesParsed, bytesEmitted, rawTokens, fallbackUsed, passthrough, teePath, rendered)
	record.VerifierRepairs = verification.repairs
	record.VerifierSkipped = verification.skipped
	e.appendStreamingHistory(record)
	result := buildStreamingResult(profile, profileConfidence, rendered, rawCombined, execResult.ExitCode, teePath, duration, fallbackUsed, fastPath, rawBytesRead, bytesParsed, bytesEmitted, verification)
	publishFinalPartial(onPartial, result)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (e *Engine) prepareStreamingExecution(
	inv Invocation,
	passthrough bool,
	onPartial func(PartialResult),
) (Invocation, Profile, []string, OutputBudget, StreamReducer, runOptions, string) {
	preparedInv, _ := e.prepareInvocation(inv)
	profile := e.match(preparedInv)
	command := preparedInv.Command
	if profile.Prepare != nil {
		command = profile.Prepare(preparedInv)
	}
	budget, _ := ResolveBudgetWithAdapter(profile, preparedInv, e.config.MaxPreviewLines, e.budgetAdapter)
	streamReducer := streamingReducer(profile, preparedInv, budget, passthrough)
	profileConfidence := normalizedProfileConfidence(profile)
	options := buildRunOptions(preparedInv, profile, passthrough, streamReducer)
	options.command = command
	options.teeOnFailure = e.config.TeeOnFailure
	options.teeDir = e.paths.TeeDir
	options.reducer = streamReducer
	options.onPreview = previewPublisher(onPartial, profile.Name, profileConfidence)
	return preparedInv, profile, command, budget, streamReducer, options, profileConfidence
}

func streamingReducer(profile Profile, inv Invocation, budget OutputBudget, passthrough bool) StreamReducer {
	if passthrough || profile.StreamRender == nil {
		return nil
	}
	return profile.StreamRender(inv, budget)
}

func normalizedProfileConfidence(profile Profile) string {
	if profile.Confidence == "" {
		return ConfidenceMedium
	}
	return profile.Confidence
}

func previewPublisher(onPartial func(PartialResult), profileName string, profileConfidence string) func(string, int, bool) {
	if onPartial == nil {
		return nil
	}
	return func(text string, bytesParsed int, done bool) {
		onPartial(PartialResult{
			ProfileName:       profileName,
			ProfileConfidence: profileConfidence,
			Display:           strings.TrimRight(text, "\n"),
			BytesParsed:       bytesParsed,
			Final:             done,
		})
	}
}

func (e *Engine) runStreamingCommand(
	ctx context.Context,
	inv Invocation,
	command []string,
	profile Profile,
	streamReducer StreamReducer,
	options runOptions,
) (runResult, Execution, FastPathDecision, string, int, int, time.Duration, error) {
	start := time.Now()
	runResult, err := runCommand(ctx, command, inv.Cwd, options)
	duration := time.Since(start)
	execResult := Execution{
		Command:  command,
		Stdout:   runResult.stdout,
		Stderr:   runResult.stderr,
		ExitCode: runResult.exitCode,
		Duration: duration,
	}
	rawCombined := combineStreams(runResult.stdout, runResult.stderr)
	rawBytesRead := runResult.stdoutBytes + runResult.stderrBytes
	rawTokens := runResult.rawTokens
	fastPath := DecideFastPath(profile, inv, bytesForFastPath(profile, runResult), rawTokens, duration, execResult.ExitCode)
	return runResult, execResult, fastPath, rawCombined, rawBytesRead, rawTokens, duration, err
}

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
) (string, bool, RecoveryPlan) {
	rendered := rawCombined
	if !passthrough {
		rendered = renderedStreamingContent(profile, preparedInv, execResult, streamReducer, rawCombined, fastPath, rawBytesRead, captureTruncated)
	}
	fallbackUsed := streamingFallbackUsed(profile, streamReducer, passthrough, rendered, rawCombined)
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
	rendered, recoveryPlan, _ = enforceCompressionContract(rendered, rawCombined, rawTokens, budget, recoveryPlan, passthrough, preparedInv.Advanced.CompressionContract)
	rendered = ensureInformativeFailureRender(profile, preparedInv, rendered, rawCombined, failureArtifactPath, execResult.ExitCode, passthrough, budget)
	rendered = preferTerseRenderForRewrittenCommand(rendered, rawCombined, execResult.ExitCode, commandRewritten, passthrough, captureTruncated)
	if !captureTruncated && shouldGuardSmallOutput(profile, passthrough) && !preparedInv.UltraCompact {
		rendered = preferRawSmallOutputForProfile(profile, rendered, rawCombined, execResult.ExitCode)
	}
	return rendered, fallbackUsed, recoveryPlan
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
// failures — the render may exceed the contract cap here on purpose.
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
// (for example kubectl-get, which is stdout-only) do not buffer the other
// stream at all — yet on failure that other stream usually carries the
// diagnostics. The tee artifact written during the run holds the full
// interleaved output; a bounded prefix is enough for a compact escape
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
) string {
	// When capture stopped at the preview limit, rawCombined is only the
	// head of the stream; compacting it would replace a correct summary
	// with the least informative part of the output.
	if passthrough || !commandRewritten || exitCode != 0 || captureTruncated {
		return rendered
	}
	renderedTokens := history.EstimateTokens(rendered)
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
		return rawCombined
	case streamReducer != nil:
		return streamReducer.Result()
	case profile.Render != nil:
		return profile.Render(preparedInv, execResult)
	default:
		return rawCombined
	}
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

func streamingFallbackUsed(profile Profile, streamReducer StreamReducer, passthrough bool, rendered string, rawCombined string) bool {
	fallbackUsed := streamReducer != nil && streamReducer.FallbackUsed()
	if strings.TrimSpace(rendered) == "" {
		fallbackUsed = !passthrough || fallbackUsed
	}
	if !passthrough && profile.Name == "passthrough" {
		fallbackUsed = true
	}
	return fallbackUsed
}

func (e *Engine) ensureStreamingArtifactPath(
	teePath string,
	exitCode int,
	rawCombined string,
	command []string,
	profile Profile,
	fallbackUsed bool,
	recoveryPlan RecoveryPlan,
	passthrough bool,
) string {
	if teePath != "" {
		if shouldPersistRecoveryArtifact(recoveryPlan, rawCombined, passthrough) {
			return teePath
		}
		if isFailureExit(profile, exitCode) && e.config.TeeOnFailure && shouldPersistFailureArtifact(profile, fallbackUsed, passthrough) {
			return teePath
		}
		_ = os.Remove(teePath)
		return ""
	}
	if rawCombined == "" {
		return ""
	}
	if shouldPersistRecoveryArtifact(recoveryPlan, rawCombined, passthrough) {
		path, teeErr := e.writeTee(rawCombined, command)
		if teeErr != nil {
			return ""
		}
		return path
	}
	if !isFailureExit(profile, exitCode) {
		return ""
	}
	if !e.config.TeeOnFailure || !shouldPersistFailureArtifact(profile, fallbackUsed, passthrough) {
		return ""
	}
	path, teeErr := e.writeTee(rawCombined, command)
	if teeErr != nil {
		return ""
	}
	return path
}

func shouldPersistFailureArtifact(profile Profile, fallbackUsed bool, passthrough bool) bool {
	if passthrough {
		return true
	}
	if profile.Name == "passthrough" || fallbackUsed {
		return true
	}
	return profile.Confidence != ConfidenceHigh
}

func appendTeeReference(rendered string, teePath string, passthrough bool) string {
	if teePath == "" || passthrough {
		return rendered
	}
	return strings.TrimRight(rendered, "\n") + "\n[full output: " + teePath + "]"
}

func streamingBytesParsed(streamReducer StreamReducer, profile Profile, execResult Execution, rawBytesRead int) int {
	bytesParsed := rawBytesRead
	if streamReducer != nil {
		bytesParsed = streamReducer.BytesParsed()
	}
	if bytesParsed <= 0 && profile.ParseBytes != nil {
		bytesParsed = profile.ParseBytes(execResult)
	}
	if bytesParsed < 0 {
		return 0
	}
	return bytesParsed
}

func buildStreamingHistoryRecord(
	inv Invocation,
	profile Profile,
	profileConfidence string,
	duration time.Duration,
	exitCode int,
	rawBytesRead int,
	bytesParsed int,
	bytesEmitted int,
	rawTokens int,
	fallbackUsed bool,
	passthrough bool,
	teePath string,
	rendered string,
) history.Record {
	commandText := strings.Join(inv.Display, " ")
	record := history.Record{
		Timestamp:          time.Now(),
		Command:            commandText,
		CommandFingerprint: history.Fingerprint(commandText),
		Profile:            profile.Name,
		ProfileConfidence:  profileConfidence,
		Cwd:                inv.Cwd,
		DurationMS:         duration.Milliseconds(),
		ExitCode:           exitCode,
		RawBytes:           rawBytesRead,
		FilteredBytes:      bytesEmitted,
		RawBytesRead:       rawBytesRead,
		BytesParsed:        bytesParsed,
		BytesEmitted:       bytesEmitted,
		RawTokens:          rawTokens,
		FilteredTokens:     history.EstimateTokens(rendered),
		FallbackUsed:       fallbackUsed,
		Passthrough:        passthrough,
		TeePath:            teePath,
	}
	record.SavedTokens = record.RawTokens - record.FilteredTokens
	if record.RawTokens > 0 {
		record.SavingsPct = float64(record.SavedTokens) * 100 / float64(record.RawTokens)
	}
	return record
}

func (e *Engine) appendStreamingHistory(record history.Record) {
	_ = e.history.Append(record)
	if record.TeePath == "" {
		return
	}
	_ = teeindex.New(e.paths.TeeDir).Append(teeindex.Entry{
		Timestamp:          record.Timestamp,
		Path:               record.TeePath,
		Command:            record.Command,
		CommandFingerprint: record.CommandFingerprint,
		Profile:            record.Profile,
		ProfileConfidence:  record.ProfileConfidence,
		Cwd:                record.Cwd,
		ExitCode:           record.ExitCode,
		DurationMS:         record.DurationMS,
		RawBytes:           record.RawBytesRead,
		RawTokens:          record.RawTokens,
	})
}

func buildStreamingResult(
	profile Profile,
	profileConfidence string,
	rendered string,
	rawCombined string,
	exitCode int,
	teePath string,
	duration time.Duration,
	fallbackUsed bool,
	fastPath FastPathDecision,
	rawBytesRead int,
	bytesParsed int,
	bytesEmitted int,
	verification retentionOutcome,
) Result {
	return Result{
		ProfileName:       profile.Name,
		ProfileConfidence: profileConfidence,
		Display:           strings.TrimRight(rendered, "\n"),
		RawCombined:       rawCombined,
		ExitCode:          exitCode,
		TeePath:           teePath,
		Duration:          duration,
		FallbackUsed:      fallbackUsed,
		BypassReason:      bypassReason(fastPath),
		LatencyWarning:    fastPath.WarnLatency,
		RawBytesRead:      rawBytesRead,
		BytesParsed:       bytesParsed,
		BytesEmitted:      bytesEmitted,
		VerifierRepairs:   verification.repairs,
		VerifierSkipped:   verification.skipped,
	}
}

// streamingCaptureIncomplete reports whether the in-memory capture is an
// honest stand-in for the raw stream. Preview-truncated buffers and streams
// that were never buffered at all (stream-preference profiles disable the
// non-preferred preview) both disqualify the capture as a verification
// source.
func streamingCaptureIncomplete(result runResult) bool {
	if result.captureTruncated {
		return true
	}
	if result.stdoutBytes > 0 && result.stdout == "" {
		return true
	}
	return result.stderrBytes > 0 && result.stderr == ""
}

func publishFinalPartial(onPartial func(PartialResult), result Result) {
	if onPartial == nil {
		return
	}
	onPartial(PartialResult{
		ProfileName:       result.ProfileName,
		ProfileConfidence: result.ProfileConfidence,
		Display:           result.Display,
		BytesParsed:       result.BytesParsed,
		Final:             true,
	})
}

const rawPreviewBytes = defaultTinyOutputBypassBytes * 2

func buildRunOptions(inv Invocation, profile Profile, passthrough bool, streamReducer StreamReducer) runOptions {
	options := runOptions{}
	hasStreamReducer := streamReducer != nil
	fullCapture := passthrough || inv.Verbose >= 3 || (!hasStreamReducer && profile.Render != nil) || profile.Capabilities.RequireFullCapture
	if !fullCapture {
		recoveryPlan := reducerRecoveryPlan(streamReducer)
		fullCapture = recoveryPlan.RequireRawCapture
	}
	applyCaptureMode(&options, fullCapture)

	if !hasStreamReducer {
		return options
	}

	applyStreamPreference(&options, profile.StreamPreference, fullCapture)
	applyEarlyCaptureStop(&options, inv, profile, fullCapture)

	return options
}

func applyCaptureMode(options *runOptions, fullCapture bool) {
	if fullCapture {
		options.captureStdout = true
		options.captureStderr = true
		return
	}
	options.stdoutPreviewBytes = rawPreviewBytes
	options.stderrPreviewBytes = rawPreviewBytes
}

func applyStreamPreference(options *runOptions, pref string, fullCapture bool) {
	switch pref {
	case StreamStdoutOnly:
		options.reduceStdoutLive = true
		disableStderrPreview(options, fullCapture)
	case StreamStderrOnly:
		options.reduceStderrLive = true
		disableStdoutPreview(options, fullCapture)
	case StreamStdoutFirst:
		options.reduceStdoutLive = true
		options.reduceStderrLater = true
		options.captureStderr = true
		disableStderrPreview(options, fullCapture)
	case StreamStderrFirst:
		options.reduceStderrLive = true
		options.reduceStdoutLater = true
		options.captureStdout = true
		disableStdoutPreview(options, fullCapture)
	default:
		options.reduceStdoutLive = true
		options.reduceStderrLive = true
	}
}

func disableStdoutPreview(options *runOptions, fullCapture bool) {
	if !fullCapture {
		options.stdoutPreviewBytes = 0
	}
}

func disableStderrPreview(options *runOptions, fullCapture bool) {
	if !fullCapture {
		options.stderrPreviewBytes = 0
	}
}

func applyEarlyCaptureStop(options *runOptions, inv Invocation, profile Profile, fullCapture bool) {
	if fullCapture || !inv.Advanced.EarlyCaptureStop {
		return
	}
	bypassBytes, bypassTokens := fastPathBypassLimits(profile, inv)
	if options.reduceStdoutLive && !options.captureStdout {
		options.stopStdoutEarly = true
		options.stdoutBypassBytes = bypassBytes
		options.stdoutBypassTokens = bypassTokens
	}
	if options.reduceStderrLive && !options.captureStderr {
		options.stopStderrEarly = true
		options.stderrBypassBytes = bypassBytes
		options.stderrBypassTokens = bypassTokens
	}
}

func fastPathBypassLimits(profile Profile, inv Invocation) (int, int) {
	if rule, ok := familyFastPathRules[fastPathRuleKey(profile, inv)]; ok {
		return rule.MaxBytes, rule.MaxTokens
	}
	return defaultTinyOutputBypassBytes, defaultTinyOutputBypassTokens
}
