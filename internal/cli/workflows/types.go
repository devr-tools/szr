package workflows

import (
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
)

type HotspotStat struct {
	Fingerprint    string   `json:"fingerprint"`
	Command        string   `json:"command"`
	Profile        string   `json:"profile"`
	Samples        int      `json:"samples"`
	RawTokens      int      `json:"raw_tokens"`
	FilteredTokens int      `json:"filtered_tokens"`
	AveragePct     float64  `json:"average_pct"`
	Failures       int      `json:"failures"`
	FailureRate    float64  `json:"failure_rate"`
	Fallbacks      int      `json:"fallbacks"`
	FallbackRate   float64  `json:"fallback_rate"`
	TeeCount       int      `json:"tee_count"`
	TeeRate        float64  `json:"tee_rate"`
	DurationP50MS  int64    `json:"duration_p50_ms"`
	DurationP95MS  int64    `json:"duration_p95_ms"`
	Signals        []string `json:"signals,omitempty"`
	CoverageScore  int      `json:"coverage_score,omitempty"`
}

type Recommendation struct {
	Kind        string               `json:"kind"`
	Priority    int                  `json:"priority"`
	Command     string               `json:"command"`
	Profile     string               `json:"profile,omitempty"`
	Samples     int                  `json:"samples"`
	Confidence  string               `json:"confidence,omitempty"`
	Reason      string               `json:"reason"`
	Action      string               `json:"action"`
	Fingerprint string               `json:"fingerprint,omitempty"`
	Direction   string               `json:"direction,omitempty"`
	Suggested   history.BudgetTarget `json:"suggested,omitempty"`
}

type replayOutput struct {
	Command           string              `json:"command,omitempty"`
	EffectiveCommand  string              `json:"effective_command,omitempty"`
	Profile           string              `json:"profile"`
	ProfileConfidence string              `json:"profile_confidence,omitempty"`
	ExitCode          int                 `json:"exit_code"`
	FallbackUsed      bool                `json:"fallback_used"`
	RawTokens         int                 `json:"raw_tokens"`
	FilteredTokens    int                 `json:"filtered_tokens"`
	SavedTokens       int                 `json:"saved_tokens"`
	SavingsPct        float64             `json:"savings_pct"`
	BytesParsed       int                 `json:"bytes_parsed"`
	BytesEmitted      int                 `json:"bytes_emitted"`
	Budget            engine.OutputBudget `json:"budget"`
	Display           string              `json:"display"`
}

type compareOutput struct {
	Command           string              `json:"command"`
	EffectiveCommand  string              `json:"effective_command"`
	Profile           string              `json:"profile"`
	ProfileConfidence string              `json:"profile_confidence,omitempty"`
	ExitCode          int                 `json:"exit_code"`
	DurationMS        int64               `json:"duration_ms"`
	FallbackUsed      bool                `json:"fallback_used"`
	RawTokens         int                 `json:"raw_tokens"`
	FilteredTokens    int                 `json:"filtered_tokens"`
	SavedTokens       int                 `json:"saved_tokens"`
	SavingsPct        float64             `json:"savings_pct"`
	BytesParsed       int                 `json:"bytes_parsed"`
	BytesEmitted      int                 `json:"bytes_emitted"`
	Budget            engine.OutputBudget `json:"budget"`
	RawPreview        string              `json:"raw_preview"`
	ReducedPreview    string              `json:"reduced_preview"`
}

type replayOptions struct {
	asJSON           bool
	commandText      string
	profileName      string
	overrideExitCode int
	overrideExitSet  bool
	overrideCwd      string
	maxLines         int
	target           string
}
