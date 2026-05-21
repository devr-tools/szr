package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"szr/internal/filters"
	"szr/internal/history"
	"szr/internal/teeindex"
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

	preparedInv, _ := e.prepareInvocation(inv)
	profile := e.match(preparedInv)
	command := preparedInv.Command
	if profile.Prepare != nil {
		command = profile.Prepare(preparedInv)
	}

	budget := ResolveBudget(profile, preparedInv, e.config.MaxPreviewLines)
	var streamReducer StreamReducer
	if !passthrough && profile.StreamRender != nil {
		streamReducer = profile.StreamRender(preparedInv, budget)
	}
	options := buildRunOptions(preparedInv, profile, passthrough, streamReducer != nil)
	if onPartial != nil {
		profileConfidence := profile.Confidence
		if profileConfidence == "" {
			profileConfidence = ConfidenceMedium
		}
		options.onPreview = func(text string, bytesParsed int, done bool) {
			onPartial(PartialResult{
				ProfileName:       profile.Name,
				ProfileConfidence: profileConfidence,
				Display:           strings.TrimRight(text, "\n"),
				BytesParsed:       bytesParsed,
				Final:             done,
			})
		}
	}

	start := time.Now()
	options.command = command
	options.teeOnFailure = e.config.TeeOnFailure
	options.teeDir = e.paths.TeeDir
	options.reducer = streamReducer
	runResult, err := runCommand(ctx, command, inv.Cwd, options)
	duration := time.Since(start)
	stdout := runResult.stdout
	stderr := runResult.stderr
	exitCode := runResult.exitCode
	rawCombined := combineStreams(stdout, stderr)
	rawBytesRead := runResult.stdoutBytes + runResult.stderrBytes
	rawTokens := runResult.rawTokens
	fastPath := DecideFastPath(profile, bytesForFastPath(profile, runResult), rawTokens, duration, exitCode)

	execResult := Execution{
		Command:  command,
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
		Duration: duration,
	}
	rendered := rawCombined
	if !passthrough {
		switch {
		case shouldApplyBypass(profile, fastPath):
			rendered = rawCombined
		case streamReducer != nil:
			rendered = streamReducer.Result()
		case profile.Render != nil:
			rendered = profile.Render(preparedInv, execResult)
		}
	}
	fallbackUsed := false
	if streamReducer != nil && streamReducer.FallbackUsed() {
		fallbackUsed = true
	}
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
		fallbackUsed = !passthrough || fallbackUsed
	}
	if !passthrough && profile.Name == "passthrough" {
		fallbackUsed = true
	}
	if shouldUseFailureEscape(profile, exitCode, passthrough, fallbackUsed) && rawCombined != "" {
		escapeBudget := ExpandBudgetForFailureEscape(budget, preparedInv)
		if escaped := filters.CompactLines(rawCombined, escapeBudget.MaxLines); strings.TrimSpace(escaped) != "" {
			rendered = escaped
		}
	}

	teePath := runResult.teePath
	if teePath == "" && exitCode != 0 && e.config.TeeOnFailure && rawCombined != "" {
		path, teeErr := e.writeTee(rawCombined, command)
		if teeErr == nil {
			teePath = path
		}
	}
	if teePath != "" && !passthrough {
		rendered = strings.TrimRight(rendered, "\n") + "\n[full output: " + teePath + "]"
	}

	bytesParsed := rawBytesRead
	if streamReducer != nil {
		bytesParsed = streamReducer.BytesParsed()
	}
	if bytesParsed <= 0 && profile.ParseBytes != nil {
		bytesParsed = profile.ParseBytes(execResult)
	}
	if bytesParsed < 0 {
		bytesParsed = 0
	}
	bytesEmitted := len(rendered)
	profileConfidence := profile.Confidence
	if profileConfidence == "" {
		profileConfidence = ConfidenceMedium
	}

	record := history.Record{
		Timestamp:          time.Now(),
		Command:            strings.Join(inv.Display, " "),
		CommandFingerprint: history.Fingerprint(strings.Join(inv.Display, " ")),
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
	_ = e.history.Append(record)
	if teePath != "" {
		_ = teeindex.New(e.paths.TeeDir).Append(teeindex.Entry{
			Timestamp:          record.Timestamp,
			Path:               teePath,
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

	result := Result{
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
	if onPartial != nil {
		onPartial(PartialResult{
			ProfileName:       result.ProfileName,
			ProfileConfidence: result.ProfileConfidence,
			Display:           result.Display,
			BytesParsed:       result.BytesParsed,
			Final:             true,
		})
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

const rawPreviewBytes = defaultTinyOutputBypassBytes * 2

func buildRunOptions(inv Invocation, profile Profile, passthrough bool, hasStreamReducer bool) runOptions {
	options := runOptions{}
	fullCapture := passthrough || inv.Verbose >= 3 || (!hasStreamReducer && profile.Render != nil) || profile.Confidence != ConfidenceHigh
	if fullCapture {
		options.captureStdout = true
		options.captureStderr = true
	} else {
		options.stdoutPreviewBytes = rawPreviewBytes
		options.stderrPreviewBytes = rawPreviewBytes
	}

	if !hasStreamReducer {
		return options
	}

	switch profile.StreamPreference {
	case StreamStdoutOnly:
		options.reduceStdoutLive = true
		if !fullCapture {
			options.stderrPreviewBytes = 0
		}
	case StreamStderrOnly:
		options.reduceStderrLive = true
		if !fullCapture {
			options.stdoutPreviewBytes = 0
		}
	case StreamStdoutFirst:
		options.reduceStdoutLive = true
		options.reduceStderrLater = true
		options.captureStderr = true
		if !fullCapture {
			options.stderrPreviewBytes = 0
		}
	case StreamStderrFirst:
		options.reduceStderrLive = true
		options.reduceStdoutLater = true
		options.captureStdout = true
		if !fullCapture {
			options.stdoutPreviewBytes = 0
		}
	default:
		options.reduceStdoutLive = true
		options.reduceStderrLive = true
	}

	return options
}
