package engine

import "time"

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

const (
	StreamAuto        = "auto"
	StreamStdoutOnly  = "stdout-only"
	StreamStderrOnly  = "stderr-only"
	StreamStdoutFirst = "stdout-first"
	StreamStderrFirst = "stderr-first"
)

type OutputBudget struct {
	MaxLines  int
	MaxBytes  int
	MaxTokens int
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

type PartialResult struct {
	ProfileName       string
	ProfileConfidence string
	Display           string
	BytesParsed       int
	Final             bool
}

type Profile struct {
	Name             string
	Description      string
	Confidence       string
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
