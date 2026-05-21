package python

import (
	"strings"

	"szr/internal/engine"
	shared "szr/internal/filters"
	pyfilter "szr/internal/filters/python"
	"szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "pytest",
			Description:      "Normalizes pytest-family commands and preserves failing test identifiers, assertion lines, and short summaries.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
			LatencyBudget:    profilekit.LatencyBudget(35),
			Match: func(inv engine.Invocation) bool {
				return isPytestCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return preparePytestCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return pyfilter.SummarizePytest(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, _ engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return pyfilter.SummarizePytest(input, maxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Recognizes direct `pytest`, `python -m pytest`, and `uv run pytest` invocations.",
				"Adds low-noise flags when the user did not already choose verbosity, traceback, color, or report-char behavior.",
				"Collapses passing chatter while keeping failing test ids, fixture errors, assertion lines, and repair-relevant file anchors.",
			},
		},
	}
}

func isPytestCommand(args []string) bool {
	switch {
	case len(args) >= 1 && args[0] == "pytest":
		return true
	case len(args) >= 3 && args[0] == "python" && args[1] == "-m" && args[2] == "pytest":
		return true
	case len(args) >= 3 && args[0] == "uv" && args[1] == "run" && args[2] == "pytest":
		return true
	default:
		return false
	}
}

func preparePytestCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	if !hasPytestVerbosityArg(command) {
		out = append(out, "-q")
	}
	if !profilekit.ContainsPrefix(command[1:], "--tb=") {
		out = append(out, "--tb=short")
	}
	if !profilekit.ContainsPrefix(command[1:], "--color=") {
		out = append(out, "--color=no")
	}
	if !hasPytestReportChars(command) {
		out = append(out, "-ra")
	}
	return out
}

func hasPytestVerbosityArg(args []string) bool {
	for _, arg := range args {
		switch {
		case arg == "-q", arg == "-qq", arg == "--quiet", arg == "-v", arg == "-vv", arg == "-vvv", arg == "--verbose":
			return true
		case strings.HasPrefix(arg, "-q"), strings.HasPrefix(arg, "-v"):
			return true
		}
	}
	return false
}

func hasPytestReportChars(args []string) bool {
	for _, arg := range args {
		if arg == "--reportchars" || strings.HasPrefix(arg, "--reportchars=") {
			return true
		}
		if strings.HasPrefix(arg, "-r") && len(arg) > 2 {
			return true
		}
	}
	return false
}
