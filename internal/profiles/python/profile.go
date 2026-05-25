package python

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	pyfilter "github.com/devr-tools/szr/internal/filters/python"
	"github.com/devr-tools/szr/internal/profilekit"
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
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return pyfilter.SummarizePytest(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Recognizes direct `pytest`, `python -m pytest`, and `uv run pytest` invocations.",
				"Adds low-noise flags when the user did not already choose verbosity, traceback, color, or report-char behavior.",
				"Collapses passing chatter while keeping failing test ids, fixture errors, assertion lines, and repair-relevant file anchors.",
			},
		},
		{
			Name:             "python-tooling",
			Description:      "Summarizes Python package, lint, and type-check tooling around actionable diagnostics.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(30),
			Match: func(inv engine.Invocation) bool {
				return isPythonToolingCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return preparePythonToolingCommand(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return pyfilter.SummarizePythonTooling(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducer(true, true, func(input string) string {
					return pyfilter.SummarizePythonTooling(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches Python package-management, lint, and type-check tooling such as `uv`, `poetry`, `pip`, `ruff`, and `mypy`.",
				"Prefers concise linter and type-check output when safe, and preserves actionable file, rule, and package-resolution failures.",
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

func isPythonToolingCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "uv":
		return !(len(args) >= 3 && args[1] == "run" && args[2] == "pytest")
	case "poetry", "pip", "pip3", "ruff", "mypy":
		return true
	case "python":
		return len(args) >= 3 && args[1] == "-m" && args[2] != "pytest" && (args[2] == "pip" || args[2] == "ruff" || args[2] == "mypy")
	default:
		return false
	}
}

func preparePythonToolingCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	switch detectPythonTool(command) {
	case "ruff":
		if profilekit.ContainsAny(command, "--output-format") || profilekit.ContainsPrefix(command, "--output-format=") {
			return command
		}
		return append(command, "--output-format", "concise")
	case "mypy":
		out := append([]string{}, command...)
		if !profilekit.ContainsAny(command, "--show-error-codes") {
			out = append(out, "--show-error-codes")
		}
		if !profilekit.ContainsAny(command, "--hide-error-context") {
			out = append(out, "--hide-error-context")
		}
		if !profilekit.ContainsAny(command, "--no-color-output") {
			out = append(out, "--no-color-output")
		}
		return out
	default:
		return command
	}
}

func detectPythonTool(command []string) string {
	if len(command) == 0 {
		return ""
	}
	switch command[0] {
	case "ruff", "mypy", "uv", "poetry", "pip", "pip3":
		if command[0] == "python" && len(command) >= 3 {
			return command[2]
		}
		if command[0] == "uv" && len(command) >= 3 && command[1] == "run" {
			return command[2]
		}
		return command[0]
	case "python":
		if len(command) >= 3 && command[1] == "-m" {
			return command[2]
		}
	}
	return ""
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
