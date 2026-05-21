package engine

type Profile struct {
	Name        string
	Description string
	Match       func(Invocation) bool
	Prepare     func(Invocation) []string
	Render      func(Invocation, Execution) string
	Explain     []string
}
