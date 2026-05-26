package engine

import (
	"time"

	"github.com/devr-tools/szr/internal/config"
)

type Invocation struct {
	Command             []string
	Display             []string
	Cwd                 string
	Verbose             int
	UltraCompact        bool
	ReasoningBudgetMode string
	Advanced            config.Advanced
}

type Execution struct {
	Command  []string
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

type Result struct {
	ProfileName       string
	ProfileConfidence string
	Display           string
	RawCombined       string
	ExitCode          int
	TeePath           string
	Duration          time.Duration
	FallbackUsed      bool
	BypassReason      string
	LatencyWarning    bool
	RawBytesRead      int
	BytesParsed       int
	BytesEmitted      int
}
