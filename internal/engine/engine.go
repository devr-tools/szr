package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"szr/internal/config"
	"szr/internal/history"
)

type Engine struct {
	config   config.Config
	paths    config.Paths
	history  *history.Store
	profiles []Profile
}

func New(cfg config.Config, paths config.Paths, store *history.Store, profiles []Profile) *Engine {
	return &Engine{
		config:   cfg,
		paths:    paths,
		history:  store,
		profiles: mergeProfiles(compileRuleProfiles(cfg), profiles),
	}
}

func (e *Engine) Profiles() []Profile {
	return append([]Profile(nil), e.profiles...)
}

func (e *Engine) Explain(inv Invocation) Profile {
	return e.match(inv)
}

func (e *Engine) Execute(ctx context.Context, inv Invocation, passthrough bool) (Result, error) {
	if len(inv.Command) == 0 {
		return Result{}, fmt.Errorf("missing command")
	}

	profile := e.match(inv)
	command := inv.Command
	if profile.Prepare != nil {
		command = profile.Prepare(inv)
	}

	budget := ResolveBudget(profile, e.config.MaxPreviewLines)
	var streamReducer StreamReducer
	if !passthrough && profile.StreamRender != nil {
		streamReducer = profile.StreamRender(inv, budget)
	}
	options := buildRunOptions(inv, profile, passthrough, streamReducer != nil)

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
			rendered = profile.Render(inv, execResult)
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

func shouldApplyBypass(profile Profile, decision FastPathDecision) bool {
	if !decision.BypassCompression {
		return false
	}
	if profile.Name == "passthrough" {
		return true
	}
	if profile.StreamRender == nil {
		return false
	}
	return profile.Confidence != ConfidenceHigh
}

func bypassReason(decision FastPathDecision) string {
	if !decision.BypassCompression {
		return ""
	}
	return decision.Reason
}

func bytesForFastPath(profile Profile, result runResult) int {
	switch profile.StreamPreference {
	case StreamStdoutOnly, StreamStdoutFirst:
		return result.stdoutBytes
	case StreamStderrOnly, StreamStderrFirst:
		return result.stderrBytes
	default:
		return result.stdoutBytes + result.stderrBytes
	}
}
