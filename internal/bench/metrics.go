package bench

import (
	"sort"
	"time"

	"szr/internal/history"
)

const defaultSamples = 9

type DurationStats struct {
	Samples int           `json:"samples"`
	Min     time.Duration `json:"min"`
	P50     time.Duration `json:"p50"`
	P95     time.Duration `json:"p95"`
	Max     time.Duration `json:"max"`
	Total   time.Duration `json:"total"`
}

type Quality struct {
	Score              int      `json:"score"`
	ActionableLines    int      `json:"actionable_lines"`
	FailureIdentifiers int      `json:"failure_identifiers"`
	PreservedFailures  int      `json:"preserved_failures"`
	FallbackRate       float64  `json:"fallback_rate"`
	ProfileConfidence  string   `json:"profile_confidence"`
	Issues             []string `json:"issues"`
}

type Expectation struct {
	ContainsOK      bool     `json:"contains_ok"`
	TokenSavingsOK  bool     `json:"token_savings_ok"`
	QualityOK       bool     `json:"quality_ok"`
	MissingContains []string `json:"missing_contains,omitempty"`
	OK              bool     `json:"ok"`
}

type Measurement struct {
	FixtureName        string        `json:"fixture_name"`
	Class              string        `json:"class"`
	ProfileName        string        `json:"profile_name"`
	CommandFingerprint string        `json:"command_fingerprint"`
	Duration           time.Duration `json:"duration"`
	Durations          DurationStats `json:"durations"`
	RawCombined        string        `json:"-"`
	Rendered           string        `json:"-"`
	RawBytes           int           `json:"raw_bytes"`
	ParsedBytes        int           `json:"parsed_bytes"`
	FilteredBytes      int           `json:"filtered_bytes"`
	EmittedBytes       int           `json:"emitted_bytes"`
	SavedBytes         int           `json:"saved_bytes"`
	ByteRatio          float64       `json:"byte_ratio"`
	ByteSavingsPct     float64       `json:"byte_savings_pct"`
	RawTokens          int           `json:"raw_tokens"`
	FilteredTokens     int           `json:"filtered_tokens"`
	SavedTokens        int           `json:"saved_tokens"`
	TokenRatio         float64       `json:"token_ratio"`
	TokenSavingsPct    float64       `json:"token_savings_pct"`
	FallbackCount      int           `json:"fallback_count"`
	FallbackRate       float64       `json:"fallback_rate_pct"`
	TeeCount           int           `json:"tee_count"`
	TeeRate            float64       `json:"tee_rate_pct"`
	FailureCount       int           `json:"failure_count"`
	FailureRate        float64       `json:"failure_rate_pct"`
	Quality            Quality       `json:"quality"`
	Expectation        Expectation   `json:"expectation"`
}

func (h *Harness) Measure(fixture Fixture) (Measurement, error) {
	return h.Benchmark(fixture, defaultSamples)
}

func (h *Harness) Benchmark(fixture Fixture, samples int) (Measurement, error) {
	if samples <= 0 {
		samples = 1
	}

	raw := fixture.RawCombined()
	durations := make([]time.Duration, 0, samples)
	fallbackCount := 0
	rendered := ""
	for i := 0; i < samples; i++ {
		start := time.Now()
		outcome, err := h.renderWithMeta(fixture)
		duration := time.Since(start)
		if err != nil {
			return Measurement{}, err
		}
		durations = append(durations, duration)
		if outcome.Fallback {
			fallbackCount++
		}
		if i == 0 {
			rendered = outcome.Rendered
		}
	}

	parsedInput := parsedProfileInput(fixture)
	measurement := Measurement{
		FixtureName:        fixture.Name,
		Class:              fixture.Class,
		ProfileName:        fixture.ProfileName,
		CommandFingerprint: commandFingerprint(fixture.Invocation.Display),
		Durations:          summarizeDurations(durations),
		RawCombined:        raw,
		Rendered:           rendered,
		RawBytes:           len(raw),
		ParsedBytes:        len(parsedInput),
		FilteredBytes:      len(rendered),
		EmittedBytes:       len(rendered),
		RawTokens:          history.EstimateTokens(raw),
		FilteredTokens:     history.EstimateTokens(rendered),
		FallbackCount:      fallbackCount,
		FallbackRate:       rate(fallbackCount, samples),
		TeeCount:           boolCount(shouldTee(fixture)),
		TeeRate:            rate(boolCount(shouldTee(fixture)), 1),
		FailureCount:       boolCount(fixture.Execution.ExitCode != 0),
		FailureRate:        rate(boolCount(fixture.Execution.ExitCode != 0), 1),
	}
	measurement.Duration = measurement.Durations.P50
	measurement.SavedBytes = measurement.RawBytes - measurement.FilteredBytes
	measurement.SavedTokens = measurement.RawTokens - measurement.FilteredTokens
	measurement.ByteRatio = ratio(measurement.FilteredBytes, measurement.RawBytes)
	measurement.TokenRatio = ratio(measurement.FilteredTokens, measurement.RawTokens)
	measurement.ByteSavingsPct = percent(measurement.SavedBytes, measurement.RawBytes)
	measurement.TokenSavingsPct = percent(measurement.SavedTokens, measurement.RawTokens)
	measurement.Quality = evaluateQuality(fixture, measurement)
	measurement.Expectation = evaluateExpectation(fixture, measurement)
	return measurement, nil
}

func summarizeDurations(values []time.Duration) DurationStats {
	if len(values) == 0 {
		return DurationStats{}
	}

	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	stats := DurationStats{
		Samples: len(sorted),
		Min:     sorted[0],
		P50:     percentileDuration(sorted, 0.50),
		P95:     percentileDuration(sorted, 0.95),
		Max:     sorted[len(sorted)-1],
	}
	for _, value := range sorted {
		stats.Total += value
	}
	return stats
}

func percentileDuration(sorted []time.Duration, pct float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	pos := int(float64(len(sorted)-1) * pct)
	if pos < 0 {
		pos = 0
	}
	if pos >= len(sorted) {
		pos = len(sorted) - 1
	}
	return sorted[pos]
}

func ratio(filtered, raw int) float64 {
	if raw <= 0 {
		return 0
	}
	return float64(filtered) / float64(raw)
}

func percent(saved, raw int) float64 {
	if raw <= 0 {
		return 0
	}
	return float64(saved) * 100 / float64(raw)
}

func rate(count, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) * 100 / float64(total)
}

func boolCount(ok bool) int {
	if ok {
		return 1
	}
	return 0
}

func shouldTee(fixture Fixture) bool {
	return fixture.Execution.ExitCode != 0 && fixture.RawCombined() != ""
}
