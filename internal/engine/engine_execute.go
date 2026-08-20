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

	preparedInv, profile, command, budget, adaptation, streamReducer, options, profileConfidence := e.prepareStreamingExecution(inv, passthrough, onPartial)
	runResult, execResult, fastPath, rawCombined, rawBytesRead, rawTokens, duration, err := e.runStreamingCommand(ctx, inv, command, profile, streamReducer, options)
	commandRewritten := commandWasRewritten(executionBaselineCommand(preparedInv), command)
	// Share token estimates across render passes in this execution.
	memo := history.TokenMemo{}
	rendered, fallbackUsed, emptyResult, recoveryPlan := renderStreamingOutput(streamingRenderInput{
		profile: profile, inv: preparedInv, exec: execResult, reducer: streamReducer, budget: budget,
		raw: rawCombined, rawTokens: rawTokens, passthrough: passthrough, fastPath: fastPath, rawBytes: rawBytesRead,
		captureTruncated: runResult.captureTruncated, commandRewritten: commandRewritten, artifactPath: runResult.teePath, memo: memo,
	})
	teePath := e.ensureStreamingArtifactPath(streamingArtifactInput{
		teePath:        runResult.teePath,
		exitCode:       execResult.ExitCode,
		rawCombined:    rawCombined,
		command:        command,
		profile:        profile,
		fallbackUsed:   fallbackUsed,
		recoveryPlan:   recoveryPlan,
		passthrough:    passthrough,
		keepRawCapture: options.retainRawCapture,
	})
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
	return e.completeStreamingExecution(streamingCompletionInput{
		inv: inv, profile: profile, profileConfidence: profileConfidence, duration: duration,
		exec: execResult, reducer: streamReducer, rawCombined: rawCombined, rawBytesRead: rawBytesRead,
		rawTokens: rawTokens, memo: memo, fallbackUsed: fallbackUsed, emptyResult: emptyResult,
		passthrough: passthrough, sourceTeePath: runResult.teePath, teePath: teePath, rendered: dedupOutcome.rendered,
		verification: verification, dedupOutcome: dedupOutcome, adaptation: adaptation, fastPath: fastPath,
		onPartial: onPartial, err: err,
	})
}

func (e *Engine) prepareStreamingExecution(
	inv Invocation,
	passthrough bool,
	onPartial func(PartialResult),
) (Invocation, Profile, []string, OutputBudget, *BudgetAdaptation, StreamReducer, runOptions, string) {
	preparedInv, _ := e.prepareInvocation(inv)
	profile := e.match(preparedInv)
	command := preparedInv.Command
	// Project-local rules are repository-controlled. They may still select a
	// renderer for an unwrapped shell command, but must not rewrite that command
	// and splice repository-controlled argv back into the user's `sh -c` string.
	projectShellRewrite := profile.Source == SourceProject &&
		preparedInv.ShellWrap != nil && preparedInv.ShellWrap.CommandArg >= 0
	if profile.Prepare != nil && !projectShellRewrite {
		command = profile.Prepare(preparedInv)
	}
	if preparedInv.ShellWrap != nil {
		// Preserve wrapper semantics: rewriting is permitted only when it can
		// be translated back into the original wrapper invocation losslessly.
		command = preparedInv.ShellWrap.execCommand(preparedInv.Command, command)
	}
	budget, adaptation := ResolveBudgetWithAdapter(profile, preparedInv, e.config.MaxPreviewLines, e.budgetAdapter)
	streamReducer := streamingReducer(profile, preparedInv, budget, passthrough)
	profileConfidence := normalizedProfileConfidence(profile)
	options := buildRunOptions(preparedInv, profile, passthrough, streamReducer)
	options.command = command
	options.teeOnFailure = e.config.TeeOnFailure
	options.teeDir = e.paths.TeeDir
	options.teeLimits = e.teeLimits()
	options.reducer = streamReducer
	options.onPreview = previewPublisher(onPartial, profile.Name, profileConfidence)
	return preparedInv, profile, command, budget, adaptation, streamReducer, options, profileConfidence
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
