package bench

import "github.com/devr-tools/szr/internal/engine"

type Fixture struct {
	Name             string
	Class            string
	Description      string
	ProfileName      string
	Invocation       engine.Invocation
	Execution        engine.Execution
	ExpectedContains []string
	MinTokenSavings  float64
	MinQualityScore  int
}

func (f Fixture) RawCombined() string {
	return engine.CombineStreams(f.Execution.Stdout, f.Execution.Stderr)
}
