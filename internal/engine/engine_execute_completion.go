package engine

import (
	"time"

	"github.com/devr-tools/szr/internal/history"
)

type streamingCompletionInput struct {
	inv               Invocation
	profile           Profile
	profileConfidence string
	duration          time.Duration
	exec              Execution
	reducer           StreamReducer
	rawCombined       string
	rawBytesRead      int
	rawTokens         int
	memo              history.TokenMemo
	fallbackUsed      bool
	emptyResult       bool
	passthrough       bool
	sourceTeePath     string
	teePath           string
	rendered          string
	verification      retentionOutcome
	dedupOutcome      sessionDedupOutcome
	adaptation        *BudgetAdaptation
	fastPath          FastPathDecision
	onPartial         func(PartialResult)
	err               error
}

func (e *Engine) completeStreamingExecution(input streamingCompletionInput) (Result, error) {
	record, bytesParsed, bytesEmitted := e.recordStreamingCompletion(input)
	result := streamingCompletionResult(input, record, bytesParsed, bytesEmitted)
	e.recordBudgetOutcome(input)
	publishFinalPartial(input.onPartial, result)
	return result, input.err
}

func (e *Engine) recordStreamingCompletion(input streamingCompletionInput) (history.Record, int, int) {
	cleanupDedupCapture(input.sourceTeePath, input.teePath)
	bytesParsed := streamingBytesParsed(input.reducer, input.profile, input.exec, input.rawBytesRead)
	bytesEmitted := len(input.rendered)
	recordInput := newStreamingCompletionHistoryInput(input)
	applyStreamingCompletionVolume(&recordInput, input, bytesParsed, bytesEmitted)
	record := buildStreamingHistoryRecord(recordInput)
	record.VerifierRepairs = input.verification.repairs
	record.VerifierSkipped = input.verification.skipped
	record.DedupRef = input.dedupOutcome.ref
	record.DeltaRef = input.dedupOutcome.deltaRef
	e.appendStreamingHistory(record)
	return record, bytesParsed, bytesEmitted
}

func newStreamingCompletionHistoryInput(input streamingCompletionInput) streamingHistoryRecordInput {
	return streamingHistoryRecordInput{
		inv:               input.inv,
		profile:           input.profile,
		profileConfidence: input.profileConfidence,
		duration:          input.duration,
		exitCode:          input.exec.ExitCode,
		fallbackUsed:      input.fallbackUsed,
		emptyResult:       input.emptyResult,
		passthrough:       input.passthrough,
		teePath:           input.teePath,
		rendered:          input.rendered,
		memo:              input.memo,
	}
}

func applyStreamingCompletionVolume(record *streamingHistoryRecordInput, input streamingCompletionInput, bytesParsed int, bytesEmitted int) {
	record.rawBytesRead = input.rawBytesRead
	record.bytesParsed = bytesParsed
	record.bytesEmitted = bytesEmitted
	record.rawTokens = trueRawTokenCount(input.rawTokens, input.rawCombined, input.memo)
}

func streamingCompletionResult(input streamingCompletionInput, record history.Record, bytesParsed int, bytesEmitted int) Result {
	result := buildStreamingResult(streamingResultInput{
		profile:           input.profile,
		profileConfidence: input.profileConfidence,
		rendered:          input.rendered,
		rawCombined:       input.rawCombined,
		exitCode:          input.exec.ExitCode,
		teePath:           input.teePath,
		duration:          input.duration,
		fallbackUsed:      input.fallbackUsed,
		fastPath:          input.fastPath,
		rawBytesRead:      input.rawBytesRead,
		bytesParsed:       bytesParsed,
		bytesEmitted:      bytesEmitted,
		verification:      input.verification,
	})
	result.RawTokens = record.RawTokens
	result.FilteredTokens = record.FilteredTokens
	result.SavedTokens = record.SavedTokens
	result.DedupRef = input.dedupOutcome.ref
	result.DeltaRef = input.dedupOutcome.deltaRef
	result.BudgetAdaptation = input.adaptation
	return result
}

func (e *Engine) recordBudgetOutcome(input streamingCompletionInput) {
	if recorder, ok := e.budgetAdapter.(interface {
		RecordOutcome(*BudgetAdaptation, string, bool, int)
	}); ok {
		recorder.RecordOutcome(input.adaptation, input.profile.Name, input.fallbackUsed, input.verification.repairs)
	}
}
