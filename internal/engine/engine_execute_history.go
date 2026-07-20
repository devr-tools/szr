package engine

import (
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/teeindex"
)

type streamingHistoryRecordInput struct {
	inv               Invocation
	profile           Profile
	profileConfidence string
	duration          time.Duration
	exitCode          int
	rawBytesRead      int
	bytesParsed       int
	bytesEmitted      int
	rawTokens         int
	fallbackUsed      bool
	emptyResult       bool
	passthrough       bool
	teePath           string
	rendered          string
	memo              history.TokenMemo
}

func buildStreamingHistoryRecord(input streamingHistoryRecordInput) history.Record {
	record := newStreamingRecordBase(input.inv, input.profile, input.profileConfidence, input.duration, input.exitCode)
	applyStreamingRecordVolume(&record, input.rawBytesRead, input.bytesParsed, input.bytesEmitted, input.rawTokens, input.memo.Estimate(input.rendered))
	applyStreamingRecordFlags(&record, input.fallbackUsed, input.emptyResult, input.passthrough, input.teePath)
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
		SessionScope:       sessionScope(),
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
		Timestamp: record.Timestamp, Path: record.TeePath, Command: record.Command,
		CommandFingerprint: record.CommandFingerprint, Profile: record.Profile, ProfileConfidence: record.ProfileConfidence,
		Cwd: record.Cwd, ExitCode: record.ExitCode, DurationMS: record.DurationMS,
		RawBytes: record.RawBytesRead, RawTokens: record.RawTokens,
	})
}
