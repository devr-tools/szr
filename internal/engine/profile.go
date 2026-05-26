package engine

import "time"

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

const (
	SourceBuiltin    = "built-in"
	SourceProject    = "project-local"
	SourcePreference = "project-preference"
	SourceFallback   = "fallback"
)

const (
	StreamAuto        = "auto"
	StreamStdoutOnly  = "stdout-only"
	StreamStderrOnly  = "stderr-only"
	StreamStdoutFirst = "stdout-first"
	StreamStderrFirst = "stderr-first"
)

const (
	StructuredModeNone           = ""
	StructuredModePreferred      = "preferred"
	StructuredModeStdoutOnly     = "stdout-only"
	StructuredModeStdoutRequired = "stdout-required"
)

const (
	FastPathBypassNever       = ""
	FastPathBypassSafeOnly    = "safe-only"
	FastPathBypassSmallOutput = "small-output"
)

type OutputBudget struct {
	MaxLines           int
	MaxBytes           int
	MaxTokens          int
	MinFailures        int
	MinAnchors         int
	MinHints           int
	NoisePrefiltering  bool
	SemanticCompaction bool
	AdaptiveBudgets    bool
	EarlyCaptureStop   bool
	AggressiveRewrites bool
}

type StreamReducer interface {
	ConsumeStdout([]byte)
	ConsumeStderr([]byte)
	Result() string
	BytesParsed() int
	FallbackUsed() bool
}

type StreamReducerDone interface {
	Done() bool
}

type StreamReducerPreview interface {
	Preview() string
}

type StreamRenderFactory func(Invocation, OutputBudget) StreamReducer

type ProfileCapabilities struct {
	StructuredMode            string
	InjectsPrepareArgs        bool
	SupportsAggressivePrepare bool
	FastPathBypass            string
	AllowFailureEscape        bool
	RequireFullCapture        bool
}

type PartialResult struct {
	ProfileName       string
	ProfileConfidence string
	Display           string
	BytesParsed       int
	Final             bool
}

type ExplainDecision struct {
	Name        string
	Description string
	Source      string
	Selected    bool
	Explain     []string
}

type PreferenceDecision struct {
	Name             string
	Description      string
	Source           string
	Applied          bool
	EffectiveCommand []string
	Explain          []string
}

type Profile struct {
	Name             string
	Description      string
	Source           string
	Confidence       string
	Capabilities     ProfileCapabilities
	StreamPreference string
	Budget           OutputBudget
	LatencyBudget    time.Duration
	Match            func(Invocation) bool
	Prepare          func(Invocation) []string
	Render           func(Invocation, Execution) string
	StreamRender     StreamRenderFactory
	ParseBytes       func(Execution) int
	Explain          []string
}
