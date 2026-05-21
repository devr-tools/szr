package engine

const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

type Profile struct {
	Name        string
	Description string
	Confidence  string
	Match       func(Invocation) bool
	Prepare     func(Invocation) []string
	Render      func(Invocation, Execution) string
	ParseBytes  func(Execution) int
	Explain     []string
}
