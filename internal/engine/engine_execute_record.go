package engine

import (
	"strings"
	"time"
)

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

type streamingResultInput struct {
	profile           Profile
	profileConfidence string
	rendered          string
	rawCombined       string
	exitCode          int
	teePath           string
	duration          time.Duration
	fallbackUsed      bool
	fastPath          FastPathDecision
	rawBytesRead      int
	bytesParsed       int
	bytesEmitted      int
	verification      retentionOutcome
}

func buildStreamingResult(input streamingResultInput) Result {
	result := newStreamingResultBase(input.profile, input.profileConfidence, input.rendered, input.rawCombined, input.exitCode, input.teePath, input.duration)
	applyStreamingResultTelemetry(&result, input.fallbackUsed, input.fastPath, input.rawBytesRead, input.bytesParsed, input.bytesEmitted, input.verification)
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
