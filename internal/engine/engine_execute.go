package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/history"
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
	commandRewritten := commandWasRewritten(executionBaselineCommand(preparedInv), command)
	// One memo spans every render pass of this execution: the passes
	// repeatedly re-estimate the same raw and rendered strings.
	memo := history.TokenMemo{}
	rendered, fallbackUsed, emptyResult, recoveryPlan := renderStreamingOutput(profile, preparedInv, execResult, streamReducer, budget, rawCombined, rawTokens, passthrough, fastPath, rawBytesRead, runResult.captureTruncated, commandRewritten, runResult.teePath, memo)
	teePath := e.ensureStreamingArtifactPath(runResult.teePath, execResult.ExitCode, rawCombined, command, profile, fallbackUsed, recoveryPlan, passthrough, options.retainRawCapture)
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
		captureComplete:     !streamingCaptureIncomplete(runResult),
		memo:                memo,
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
		memo:              memo,
	})
	rendered = enforceFinalNeverWorseThanRaw(rendered, rawCombined, passthrough, !streamingCaptureIncomplete(runResult), preparedInv.UltraCompact, memo)
	dedupOutcome := e.applySessionDedup(sessionDedupInput{
		rendered:        rendered,
		rawCombined:     rawCombined,
		rawCapturePath:  runResult.teePath,
		captureComplete: !streamingCaptureIncomplete(runResult),
		exitCode:        execResult.ExitCode,
		failureExit:     isFailureExit(profile, execResult.ExitCode),
		passthrough:     passthrough,
		inv:             preparedInv,
		budget:          budget,
		commandText:     strings.Join(inv.Display, " "),
	})
	rendered = dedupOutcome.rendered
	cleanupDedupCapture(runResult.teePath, teePath)
	bytesParsed := streamingBytesParsed(streamReducer, profile, execResult, rawBytesRead)
	bytesEmitted := len(rendered)
	// The record's raw-token basis must use the same estimator as the
	// filtered count whenever the capture is authoritative; otherwise a
	// display equal to raw could still score negative savings.
	record := buildStreamingHistoryRecord(inv, profile, profileConfidence, duration, execResult.ExitCode, rawBytesRead, bytesParsed, bytesEmitted, trueRawTokenCount(rawTokens, rawCombined, memo), fallbackUsed, emptyResult, passthrough, teePath, rendered, memo)
	record.VerifierRepairs = verification.repairs
	record.VerifierSkipped = verification.skipped
	record.DedupRef = dedupOutcome.ref
	record.DeltaRef = dedupOutcome.deltaRef
	e.appendStreamingHistory(record)
	result := buildStreamingResult(profile, profileConfidence, rendered, rawCombined, execResult.ExitCode, teePath, duration, fallbackUsed, fastPath, rawBytesRead, bytesParsed, bytesEmitted, verification)
	result.DedupRef = dedupOutcome.ref
	result.DeltaRef = dedupOutcome.deltaRef
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
	if preparedInv.ShellWrap != nil {
		// Matching ran against the unwrapped inner command; execution must
		// run the user's wrapper argv, with Prepare rewrites translated back
		// into the command string only when that is lossless.
		command = preparedInv.ShellWrap.execCommand(preparedInv.Command, command)
	}
	budget, _ := ResolveBudgetWithAdapter(profile, preparedInv, e.config.MaxPreviewLines, e.budgetAdapter)
	streamReducer := streamingReducer(profile, preparedInv, budget, passthrough)
	profileConfidence := normalizedProfileConfidence(profile)
	options := buildRunOptions(preparedInv, profile, passthrough, streamReducer)
	options.command = command
	options.teeOnFailure = e.config.TeeOnFailure
	options.teeDir = e.paths.TeeDir
	options.teeLimits = e.teeLimits()
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
