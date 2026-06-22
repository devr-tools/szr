package workflows

type DiscoverSummary struct {
	Records             int `json:"records"`
	RecommendationCount int `json:"recommendation_count"`
	HotspotCount        int `json:"hotspot_count"`
}

type DiscoverOpportunity struct {
	Recommendation
	Signals       []string `json:"signals,omitempty"`
	CoverageScore int      `json:"coverage_score,omitempty"`
	AveragePct    float64  `json:"average_pct,omitempty"`
	FallbackRate  float64  `json:"fallback_rate,omitempty"`
	FailureRate   float64  `json:"failure_rate,omitempty"`
	TeeRate       float64  `json:"tee_rate,omitempty"`
	DurationP95MS int64    `json:"duration_p95_ms,omitempty"`
}

type DiscoverReport struct {
	Summary       DiscoverSummary       `json:"summary"`
	Opportunities []DiscoverOpportunity `json:"opportunities"`
}

type discoverHotspotIndex struct {
	byFingerprint map[string]HotspotStat
	byFamily      map[string]HotspotStat
}
