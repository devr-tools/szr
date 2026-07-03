package history

import "time"

type Record struct {
	Timestamp          time.Time `json:"timestamp"`
	Command            string    `json:"command"`
	CommandFingerprint string    `json:"command_fingerprint,omitempty"`
	Profile            string    `json:"profile"`
	ProfileConfidence  string    `json:"profile_confidence,omitempty"`
	Cwd                string    `json:"cwd"`
	DurationMS         int64     `json:"duration_ms"`
	ExitCode           int       `json:"exit_code"`
	RawBytes           int       `json:"raw_bytes"`
	FilteredBytes      int       `json:"filtered_bytes"`
	RawBytesRead       int       `json:"raw_bytes_read,omitempty"`
	BytesParsed        int       `json:"bytes_parsed,omitempty"`
	BytesEmitted       int       `json:"bytes_emitted,omitempty"`
	RawTokens          int       `json:"raw_tokens"`
	FilteredTokens     int       `json:"filtered_tokens"`
	SavedTokens        int       `json:"saved_tokens"`
	SavingsPct         float64   `json:"savings_pct"`
	FallbackUsed       bool      `json:"fallback_used,omitempty"`
	Passthrough        bool      `json:"passthrough,omitempty"`
	VerifierRepairs    int       `json:"verifier_repairs,omitempty"`
	VerifierSkipped    bool      `json:"verifier_skipped,omitempty"`
	DedupRef           string    `json:"dedup_ref,omitempty"`
	DeltaRef           string    `json:"delta_ref,omitempty"`
	TeePath            string    `json:"tee_path,omitempty"`
}

type Summary struct {
	Commands            int                `json:"commands"`
	AveragePct          float64            `json:"average_pct"`
	SavedTokens         int                `json:"saved_tokens"`
	RawTokens           int                `json:"raw_tokens"`
	FilteredTokens      int                `json:"filtered_tokens"`
	TotalDurationMS     int64              `json:"total_duration_ms"`
	Failures            int                `json:"failures"`
	FailureRate         float64            `json:"failure_rate"`
	Fallbacks           int                `json:"fallbacks"`
	FallbackRate        float64            `json:"fallback_rate"`
	TeeCount            int                `json:"tee_count"`
	TeeRate             float64            `json:"tee_rate"`
	PassthroughCommands int                `json:"passthrough_commands,omitempty"`
	PassthroughTokens   int                `json:"passthrough_tokens,omitempty"`
	FilteredSavingsPct  float64            `json:"filtered_savings_pct"`
	DurationP50MS       int64              `json:"duration_p50_ms"`
	DurationP95MS       int64              `json:"duration_p95_ms"`
	RawBytesRead        int                `json:"raw_bytes_read"`
	BytesParsed         int                `json:"bytes_parsed"`
	BytesEmitted        int                `json:"bytes_emitted"`
	TopCommands         []CommandStat      `json:"top_commands"`
	Recent              []Record           `json:"recent"`
	Profiles            map[string]int     `json:"profiles"`
	ProfileStats        []ProfileStat      `json:"profile_stats"`
	CommandHotspots     []CommandHotspot   `json:"command_hotspots"`
	FingerprintHotspots []FingerprintStat  `json:"fingerprint_hotspots"`
	BudgetSuggestions   []BudgetSuggestion `json:"budget_suggestions"`
}

type CommandStat struct {
	Command        string  `json:"command"`
	Count          int     `json:"count"`
	AveragePct     float64 `json:"average_pct"`
	SavedTokens    int     `json:"saved_tokens"`
	RawTokens      int     `json:"raw_tokens"`
	FilteredTokens int     `json:"filtered_tokens"`
}

type ProfileStat struct {
	Name           string  `json:"name"`
	Confidence     string  `json:"confidence,omitempty"`
	Commands       int     `json:"commands"`
	AveragePct     float64 `json:"average_pct"`
	SavedTokens    int     `json:"saved_tokens"`
	RawTokens      int     `json:"raw_tokens"`
	FilteredTokens int     `json:"filtered_tokens"`
	Failures       int     `json:"failures"`
	FailureRate    float64 `json:"failure_rate"`
	Fallbacks      int     `json:"fallbacks"`
	FallbackRate   float64 `json:"fallback_rate"`
	TeeCount       int     `json:"tee_count"`
	TeeRate        float64 `json:"tee_rate"`
	DurationP50MS  int64   `json:"duration_p50_ms"`
	DurationP95MS  int64   `json:"duration_p95_ms"`
}

type FingerprintStat struct {
	Fingerprint   string  `json:"fingerprint"`
	Command       string  `json:"command"`
	Profile       string  `json:"profile"`
	Commands      int     `json:"commands"`
	AveragePct    float64 `json:"average_pct"`
	DurationP50MS int64   `json:"duration_p50_ms"`
	DurationP95MS int64   `json:"duration_p95_ms"`
}

type CommandHotspot struct {
	Command       string  `json:"command"`
	Profile       string  `json:"profile"`
	Commands      int     `json:"commands"`
	AveragePct    float64 `json:"average_pct"`
	FailureRate   float64 `json:"failure_rate"`
	FallbackRate  float64 `json:"fallback_rate"`
	DurationP50MS int64   `json:"duration_p50_ms"`
	DurationP95MS int64   `json:"duration_p95_ms"`
}
