package engine

import (
	"context"
	"fmt"
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
	rendered, fallbackUsed, recoveryPlan := renderStreamingOutput(profile, preparedInv, execResult, streamReducer, budget, rawCombined, passthrough, fastPath)
	teePath := e.ensureStreamingArtifactPath(runResult.teePath, execResult.ExitCode, rawCombined, command, profile, fallbackUsed, recoveryPlan, passthrough)
	rendered = finalizeRenderedDisplay(rendered, rawCombined, budget, recoveryPlan, teePath, passthrough, preparedInv.Advanced.CompactArtifactRefs, preparedInv.Advanced.CompressionContract, shouldGuardSmallOutput(profile, passthrough))
	bytesParsed := streamingBytesParsed(streamReducer, profile, execResult, rawBytesRead)
	bytesEmitted := len(rendered)
	record := buildStreamingHistoryRecord(inv, profile, profileConfidence, duration, execResult.ExitCode, rawBytesRead, bytesParsed, bytesEmitted, rawTokens, fallbackUsed, teePath, rendered)
	e.appendStreamingHistory(record)
	result := buildStreamingResult(profile, profileConfidence, rendered, rawCombined, execResult.ExitCode, teePath, duration, fallbackUsed, fastPath, rawBytesRead, bytesParsed, bytesEmitted)
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
	passthrough bool,
	fastPath FastPathDecision,
) (string, bool, RecoveryPlan) {
	rendered := rawCombined
	if !passthrough {
		rendered = renderedStreamingContent(profile, preparedInv, execResult, streamReducer, rawCombined, fastPath)
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
	rendered, recoveryPlan, _ = enforceCompressionContract(rendered, rawCombined, budget, recoveryPlan, passthrough, preparedInv.Advanced.CompressionContract)
	if shouldGuardSmallOutput(profile, passthrough) {
		rendered = preferRawSmallOutput(rendered, rawCombined)
	}
	return rendered, fallbackUsed, recoveryPlan
}

func renderedStreamingContent(
	profile Profile,
	preparedInv Invocation,
	execResult Execution,
	streamReducer StreamReducer,
	rawCombined string,
	fastPath FastPathDecision,
) string {
	switch {
	case shouldApplyBypass(profile, fastPath):
		return rawCombined
	case streamReducer != nil:
		return streamReducer.Result()
	case profile.Render != nil:
		return profile.Render(preparedInv, execResult)
	default:
		return rawCombined
	}
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
		if exitCode != 0 && e.config.TeeOnFailure && shouldPersistFailureArtifact(profile, fallbackUsed, passthrough) {
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
	if exitCode == 0 {
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
	}
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
