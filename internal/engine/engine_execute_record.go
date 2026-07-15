package engine

import (
	"os"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/teeindex"
)

func (e *Engine) ensureStreamingArtifactPath(
	teePath string,
	exitCode int,
	rawCombined string,
	command []string,
	profile Profile,
	fallbackUsed bool,
	recoveryPlan RecoveryPlan,
	passthrough bool,
	keepRawCapture bool,
) string {
	if teePath != "" {
		return e.retainExistingTeeArtifact(teePath, exitCode, rawCombined, profile, fallbackUsed, recoveryPlan, passthrough, keepRawCapture)
	}
	return e.writeDeferredTeeArtifact(rawCombined, exitCode, command, profile, fallbackUsed, recoveryPlan, passthrough)
}

// retainExistingTeeArtifact decides whether the capture written during the run
// stays on disk or is removed once no retention rule claims it.
func (e *Engine) retainExistingTeeArtifact(
	teePath string,
	exitCode int,
	rawCombined string,
	profile Profile,
	fallbackUsed bool,
	recoveryPlan RecoveryPlan,
	passthrough bool,
	keepRawCapture bool,
) string {
	if shouldPersistRecoveryArtifact(recoveryPlan, rawCombined, passthrough) {
		return teePath
	}
	if isFailureExit(profile, exitCode) && e.config.TeeOnFailure && shouldPersistFailureArtifact(profile, fallbackUsed, passthrough) {
		return teePath
	}
	// Session dedup still needs the capture file; cleanupDedupCapture
	// removes it after the dedup step has archived what it needs.
	if !keepRawCapture {
		_ = os.Remove(teePath)
	}
	return ""
}

// writeDeferredTeeArtifact persists the in-memory capture after the run when
// no capture file was streamed to disk but a retention rule wants one.
func (e *Engine) writeDeferredTeeArtifact(
	rawCombined string,
	exitCode int,
	command []string,
	profile Profile,
	fallbackUsed bool,
	recoveryPlan RecoveryPlan,
	passthrough bool,
) string {
	if rawCombined == "" {
		return ""
	}
	if shouldPersistRecoveryArtifact(recoveryPlan, rawCombined, passthrough) {
		return e.writeTeeArtifact(rawCombined, command)
	}
	if !isFailureExit(profile, exitCode) {
		return ""
	}
	if !e.config.TeeOnFailure || !shouldPersistFailureArtifact(profile, fallbackUsed, passthrough) {
		return ""
	}
	return e.writeTeeArtifact(rawCombined, command)
}

func (e *Engine) writeTeeArtifact(rawCombined string, command []string) string {
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
	emptyResult bool,
	passthrough bool,
	teePath string,
	rendered string,
	memo history.TokenMemo,
) history.Record {
	record := newStreamingRecordBase(inv, profile, profileConfidence, duration, exitCode)
	applyStreamingRecordVolume(&record, rawBytesRead, bytesParsed, bytesEmitted, rawTokens, memo.Estimate(rendered))
	applyStreamingRecordFlags(&record, fallbackUsed, emptyResult, passthrough, teePath)
	return record
}

func newStreamingRecordBase(inv Invocation, profile Profile, profileConfidence string, duration time.Duration, exitCode int) history.Record {
	commandText := strings.Join(inv.Display, " ")
	return history.Record{
		Timestamp:          time.Now(),
		Command:            commandText,
		CommandFingerprint: history.Fingerprint(commandText),
		Profile:            profile.Name,
		ProfileConfidence:  profileConfidence,
		Cwd:                inv.Cwd,
		DurationMS:         duration.Milliseconds(),
		ExitCode:           exitCode,
	}
}

func applyStreamingRecordVolume(record *history.Record, rawBytesRead int, bytesParsed int, bytesEmitted int, rawTokens int, filteredTokens int) {
	record.RawBytes = rawBytesRead
	record.FilteredBytes = bytesEmitted
	record.RawBytesRead = rawBytesRead
	record.BytesParsed = bytesParsed
	record.BytesEmitted = bytesEmitted
	record.RawTokens = rawTokens
	record.FilteredTokens = filteredTokens
	record.SavedTokens = rawTokens - filteredTokens
	if rawTokens > 0 {
		record.SavingsPct = float64(record.SavedTokens) * 100 / float64(rawTokens)
	}
}

func applyStreamingRecordFlags(record *history.Record, fallbackUsed bool, emptyResult bool, passthrough bool, teePath string) {
	record.FallbackUsed = fallbackUsed
	record.EmptyResult = emptyResult
	record.Passthrough = passthrough
	record.TeePath = teePath
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
	result := newStreamingResultBase(profile, profileConfidence, rendered, rawCombined, exitCode, teePath, duration)
	applyStreamingResultTelemetry(&result, fallbackUsed, fastPath, rawBytesRead, bytesParsed, bytesEmitted, verification)
	return result
}

func newStreamingResultBase(profile Profile, profileConfidence string, rendered string, rawCombined string, exitCode int, teePath string, duration time.Duration) Result {
	return Result{
		ProfileName:       profile.Name,
		ProfileConfidence: profileConfidence,
		Display:           strings.TrimRight(rendered, "\n"),
		RawCombined:       rawCombined,
		ExitCode:          exitCode,
		TeePath:           teePath,
		Duration:          duration,
	}
}

func applyStreamingResultTelemetry(result *Result, fallbackUsed bool, fastPath FastPathDecision, rawBytesRead int, bytesParsed int, bytesEmitted int, verification retentionOutcome) {
	result.FallbackUsed = fallbackUsed
	result.BypassReason = bypassReason(fastPath)
	result.LatencyWarning = fastPath.WarnLatency
	result.RawBytesRead = rawBytesRead
	result.BytesParsed = bytesParsed
	result.BytesEmitted = bytesEmitted
	result.VerifierRepairs = verification.repairs
	result.VerifierSkipped = verification.skipped
}

// streamingCaptureIncomplete reports whether the in-memory capture is an
// honest stand-in for the raw stream. Preview-truncated buffers and streams
// whose preview overflowed both disqualify the capture as a verification
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
	options.retainRawCapture = sessionDedupWantsRawCapture(inv, passthrough)
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

// applyStreamPreference routes the live streams into the reducer. The
// non-reduced stream keeps its bounded preview: when the preferred stream is
// empty the engine renders a compact view of the other stream instead of
// escalating an empty render to a raw fallback (kubectl writes "No resources
// found ..." to stderr while its reducer is stdout-only).
func applyStreamPreference(options *runOptions, pref string, fullCapture bool) {
	switch pref {
	case StreamStdoutOnly:
		options.reduceStdoutLive = true
	case StreamStderrOnly:
		options.reduceStderrLive = true
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
