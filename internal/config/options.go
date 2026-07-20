package config

type Advanced struct {
	AggressivePrepareRewrites bool `json:"aggressive_prepare_rewrites"`
	NoisePrefiltering         bool `json:"noise_prefiltering"`
	AdaptiveBudgets           bool `json:"adaptive_budgets"`
	EarlyCaptureStop          bool `json:"early_capture_stop"`
	SemanticCompaction        bool `json:"semantic_compaction"`
	CompressionContract       bool `json:"compression_contract"`
	CompactArtifactRefs       bool `json:"compact_artifact_refs"`
	RetentionVerifier         bool `json:"retention_verifier"`
	SessionDedup              bool `json:"session_dedup"`
	SessionDedupWindowMinutes int  `json:"session_dedup_window_minutes"`
	DeltaRender               bool `json:"delta_render"`
	ProjectFilters            bool `json:"project_filters"`
}

const DefaultSessionDedupWindowMinutes = 30

const (
	DefaultTeeMaxFileMB   = 4
	DefaultTeeMaxDirFiles = 200
	DefaultTeeMaxDirMB    = 256
)

const DefaultCostRatePerMtok = 3.0

type UpdateCheck struct {
	Enabled       bool `json:"enabled"`
	IntervalHours int  `json:"interval_hours"`
	AutoUpdate    bool `json:"auto_update"`
}
