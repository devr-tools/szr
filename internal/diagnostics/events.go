// Package diagnostics records sanitized local execution events for consumers
// such as agent integrations and terminal dashboards.
package diagnostics

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const SchemaVersion = 1

const (
	EventRunProgress            = "run_progress"
	EventRunFinal               = "run_final"
	EventProviderUsageAggregate = "provider_usage_aggregate"
)

// Event intentionally contains only allowlisted operational measurements.
// It must not grow to include command text, working directories, rendered
// output, tee paths, agent/provider session identifiers, or user payload.
// GatewayCorrelationID is the sole exception: it is an opaque, gateway-issued
// join value for aggregate billing counters, never a session identifier.
type Event struct {
	Version           int       `json:"version"`
	Type              string    `json:"type"`
	RunID             string    `json:"run_id"`
	Timestamp         time.Time `json:"timestamp"`
	Profile           string    `json:"profile,omitempty"`
	ProfileConfidence string    `json:"profile_confidence,omitempty"`
	BytesParsed       int       `json:"bytes_parsed,omitempty"`
	RawBytesRead      int       `json:"raw_bytes_read,omitempty"`
	BytesEmitted      int       `json:"bytes_emitted,omitempty"`
	RawTokensEst      int       `json:"raw_tokens_est,omitempty"`
	EmittedTokensEst  int       `json:"emitted_tokens_est,omitempty"`
	SavedTokensEst    int       `json:"saved_tokens_est,omitempty"`
	DurationMS        int64     `json:"duration_ms,omitempty"`
	ExitClass         string    `json:"exit_class,omitempty"`
	FallbackUsed      bool      `json:"fallback_used,omitempty"`
	EmptyResult       bool      `json:"empty_result,omitempty"`
	Passthrough       bool      `json:"passthrough,omitempty"`
	VerifierRepairs   int       `json:"verifier_repairs,omitempty"`
	VerifierSkipped   bool      `json:"verifier_skipped,omitempty"`
	DedupUsed         bool      `json:"dedup_used,omitempty"`
	DeltaUsed         bool      `json:"delta_used,omitempty"`
	// GatewayCorrelationID is an opaque, gateway-issued join key. It is not an
	// agent, user, project, or provider session identifier.
	GatewayCorrelationID string `json:"gateway_correlation_id,omitempty"`
	AggregateRunCount    int    `json:"aggregate_run_count,omitempty"`
	ProviderInputTokens  int    `json:"provider_input_tokens,omitempty"`
	ProviderCacheTokens  int    `json:"provider_cache_tokens,omitempty"`
	ProviderOutputTokens int    `json:"provider_output_tokens,omitempty"`
}

// ProviderUsageAggregate returns the only diagnostics event intended to carry
// provider-reported billing totals. Correlation must use a gateway-issued,
// opaque value; callers must never place a provider or agent session ID here.
// The aggregate contains counters only, never prompts, transcripts, commands,
// paths, model names, or provider account identifiers.
func ProviderUsageAggregate(correlationID string, runs, rawTokensEst, emittedTokensEst, inputTokens, cacheTokens, outputTokens int) Event {
	return Event{
		Version:              SchemaVersion,
		Type:                 EventProviderUsageAggregate,
		GatewayCorrelationID: correlationID,
		Timestamp:            time.Now().UTC(),
		AggregateRunCount:    nonNegative(runs),
		RawTokensEst:         nonNegative(rawTokensEst),
		EmittedTokensEst:     nonNegative(emittedTokensEst),
		SavedTokensEst:       nonNegative(rawTokensEst - emittedTokensEst),
		ProviderInputTokens:  nonNegative(inputTokens),
		ProviderCacheTokens:  nonNegative(cacheTokens),
		ProviderOutputTokens: nonNegative(outputTokens),
	}
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

// NewRunID creates an opaque correlation id for local event consumers. It is
// deliberately unrelated to command, project, user, or agent identifiers.
func NewRunID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}
