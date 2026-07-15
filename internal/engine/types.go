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
	Classification      Classification
	// ShellWrap is set when Command was unwrapped from a shell `-c` wrapper
	// or a transparent prefix wrapper (env/nice/command/time) for
	// classification and matching only; execution still runs the original
	// wrapper argv (see ShellWrap.execCommand).
	ShellWrap *ShellWrap
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
	// VerifierRepairs counts critical raw lines the retention verifier
	// appended after the render dropped their identifying tokens.
	VerifierRepairs int
	// VerifierSkipped reports that retention verification could not run
	// because the capture was incomplete and no artifact held the full raw
	// stream.
	VerifierSkipped bool
	// DedupRef is the short session-dedup reference emitted in place of the
	// render when the output was byte-identical to a recent run.
	DedupRef string
	// DeltaRef is the baseline reference behind a delta digest render, set
	// when the run rendered as a change digest against the previous run.
	DeltaRef string
}

type Classification struct {
	Command ClassifiedCommand
	Display ClassifiedCommand
}

type ClassifiedCommand struct {
	Head       string
	Subcommand string
	Git        GitCommandFacts
	JavaScript JavaScriptCommandFacts
}

type GitCommandFacts struct {
	StatusFormatRequested bool
	LogFormatRequested    bool
	DiffFormatRequested   bool
	DiffNoPatchRequested  bool
}

type JavaScriptCommandFacts struct {
	IsPackageManagerTest bool
	IsWorkspaceCommand   bool
	IsNodeEval           bool
	Runner               string
	StructuredMode       bool
}
