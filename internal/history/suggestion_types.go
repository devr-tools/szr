package history

import "time"

type BudgetSuggestionOptions struct {
	Limit      int `json:"limit,omitempty"`
	Lookback   int `json:"lookback,omitempty"`
	MinSamples int `json:"min_samples,omitempty"`
}

type BudgetSuggestionDirection string

const (
	BudgetSuggestionTighten BudgetSuggestionDirection = "tighten"
	BudgetSuggestionLoosen  BudgetSuggestionDirection = "loosen"
)

type BudgetSuggestionReason string

const (
	BudgetSuggestionNoisy                 BudgetSuggestionReason = "noisy"
	BudgetSuggestionAggressiveCompression BudgetSuggestionReason = "aggressive_compression"
	BudgetSuggestionFallbackHeavy         BudgetSuggestionReason = "fallback_heavy"
)

type BudgetTarget struct {
	MaxLines  int `json:"max_lines"`
	MaxBytes  int `json:"max_bytes"`
	MaxTokens int `json:"max_tokens"`
}

type BudgetSuggestionEvidence struct {
	AverageSavingsPct    float64 `json:"average_savings_pct"`
	FallbackRate         float64 `json:"fallback_rate"`
	FailureRate          float64 `json:"failure_rate"`
	MedianRawTokens      int     `json:"median_raw_tokens"`
	P95RawTokens         int     `json:"p95_raw_tokens"`
	MedianFilteredTokens int     `json:"median_filtered_tokens"`
	P95FilteredTokens    int     `json:"p95_filtered_tokens"`
	MedianBytesEmitted   int     `json:"median_bytes_emitted"`
	P95BytesEmitted      int     `json:"p95_bytes_emitted"`
}

type BudgetSuggestion struct {
	Fingerprint string                    `json:"fingerprint"`
	Command     string                    `json:"command"`
	Profile     string                    `json:"profile"`
	Samples     int                       `json:"samples"`
	Direction   BudgetSuggestionDirection `json:"direction"`
	Reason      BudgetSuggestionReason    `json:"reason"`
	Confidence  string                    `json:"confidence"`
	Scale       float64                   `json:"scale"`
	Suggested   BudgetTarget              `json:"suggested"`
	Evidence    BudgetSuggestionEvidence  `json:"evidence"`
	FirstSeen   time.Time                 `json:"first_seen"`
	LastSeen    time.Time                 `json:"last_seen"`
}

type budgetSuggestionAccumulator struct {
	fingerprint    string
	command        string
	lastSeen       time.Time
	firstSeen      time.Time
	profileCounts  map[string]int
	rawTokens      []int
	filteredTokens []int
	emittedBytes   []int
	savedPct       float64
	samples        int
	failures       int
	fallbacks      int
}

type budgetSuggestionCandidate struct {
	suggestion BudgetSuggestion
	severity   float64
}
