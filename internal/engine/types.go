package engine

import "time"

type Invocation struct {
	Command      []string
	Display      []string
	Cwd          string
	Verbose      int
	UltraCompact bool
}

type Execution struct {
	Command  []string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

type Result struct {
	ProfileName string
	Display     string
	RawCombined string
	ExitCode    int
	TeePath     string
	Duration    time.Duration
}
