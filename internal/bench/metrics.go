package bench

import (
	"time"

	"szr/internal/history"
)

type Measurement struct {
	FixtureName     string
	Class           string
	ProfileName     string
	Duration        time.Duration
	RawCombined     string
	Rendered        string
	RawBytes        int
	FilteredBytes   int
	SavedBytes      int
	ByteRatio       float64
	ByteSavingsPct  float64
	RawTokens       int
	FilteredTokens  int
	SavedTokens     int
	TokenRatio      float64
	TokenSavingsPct float64
}

func (h *Harness) Measure(fixture Fixture) (Measurement, error) {
	raw := fixture.RawCombined()
	start := time.Now()
	rendered, err := h.Render(fixture)
	duration := time.Since(start)
	if err != nil {
		return Measurement{}, err
	}

	measurement := Measurement{
		FixtureName:    fixture.Name,
		Class:          fixture.Class,
		ProfileName:    fixture.ProfileName,
		Duration:       duration,
		RawCombined:    raw,
		Rendered:       rendered,
		RawBytes:       len(raw),
		FilteredBytes:  len(rendered),
		RawTokens:      history.EstimateTokens(raw),
		FilteredTokens: history.EstimateTokens(rendered),
	}
	measurement.SavedBytes = measurement.RawBytes - measurement.FilteredBytes
	measurement.SavedTokens = measurement.RawTokens - measurement.FilteredTokens
	measurement.ByteRatio = ratio(measurement.FilteredBytes, measurement.RawBytes)
	measurement.TokenRatio = ratio(measurement.FilteredTokens, measurement.RawTokens)
	measurement.ByteSavingsPct = percent(measurement.SavedBytes, measurement.RawBytes)
	measurement.TokenSavingsPct = percent(measurement.SavedTokens, measurement.RawTokens)
	return measurement, nil
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
