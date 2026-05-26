package tabular

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	tabularfilter "github.com/devr-tools/szr/internal/filters/tabular"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "csv-tabular",
			Description:      "Summarizes wide tabular CLI output into row-oriented key fields.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return matchesCSVTabular(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareCSVTabular(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return tabularfilter.SummarizeWideTable(exec.Stdout+"\n"+exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(true, true, func(input string) string {
					return tabularfilter.SummarizeWideTable(input, budget.MaxLines)
				}, func(input string) (string, string, bool) {
					return tabularfilter.WideTableRecoveryInfo(input, budget.MaxLines)
				})
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches common table-heavy commands such as `ps`, `df`, `du`, `systemctl list-units`, `helm list`, and explicit `kubectl get -o wide`.",
				"Adds stable formatting flags where the tool already supports them and reduces wide tables into row-oriented summaries instead of replaying every column verbatim.",
			},
		},
	}
}

func matchesCSVTabular(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch {
	case args[0] == "ps":
		return true
	case args[0] == "df":
		return true
	case args[0] == "du":
		return true
	case args[0] == "systemctl":
		return matchesSystemctlList(args)
	case len(args) >= 2 && args[0] == "helm" && args[1] == "list":
		return true
	case matchesKubectlGetWide(args):
		return true
	default:
		return false
	}
}

func prepareCSVTabular(command []string) []string {
	if len(command) == 0 {
		return command
	}

	switch {
	case command[0] == "ps":
		if hasAnyFlag(command[1:], "-o", "-O") || hasPrefix(command[1:], "-o=") {
			return command
		}
		return append(command, "-eo", "pid,ppid,user,%cpu,%mem,etime,command")
	case command[0] == "df":
		if hasAnyFlag(command[1:], "-P", "--portability", "--output", "-i", "-T") || hasPrefix(command[1:], "--output=") {
			return command
		}
		return append(command, "-P", "-k")
	case command[0] == "du":
		if hasAnyFlag(command[1:], "-h", "-k", "-m", "-g", "-b", "--bytes", "--human-readable") || hasPrefix(command[1:], "--block-size=") {
			return command
		}
		return insertAfterCommand(command, "-k")
	case command[0] == "systemctl" && matchesSystemctlList(command):
		out := append([]string{}, command...)
		if !hasAnyFlag(out[1:], "--plain") {
			out = append(out, "--plain")
		}
		if !hasAnyFlag(out[1:], "--no-pager") {
			out = append(out, "--no-pager")
		}
		return out
	default:
		return command
	}
}

func matchesSystemctlList(args []string) bool {
	return len(args) >= 2 && args[0] == "systemctl" && (args[1] == "list-units" || args[1] == "list-unit-files")
}

func matchesKubectlGetWide(args []string) bool {
	if len(args) < 4 || args[0] != "kubectl" {
		return false
	}
	if firstNonFlag(args[1:]) != "get" {
		return false
	}
	return hasWideOutput(args)
}

func firstNonFlag(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func hasWideOutput(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" || arg == "--output" {
			if i+1 < len(args) && args[i+1] == "wide" {
				return true
			}
			continue
		}
		if arg == "-owide" || strings.HasPrefix(arg, "-o=") && strings.TrimPrefix(arg, "-o=") == "wide" {
			return true
		}
		if strings.HasPrefix(arg, "--output=") && strings.TrimPrefix(arg, "--output=") == "wide" {
			return true
		}
	}
	return false
}

func hasAnyFlag(args []string, needles ...string) bool {
	return profilekit.ContainsAny(args, needles...)
}

func hasPrefix(args []string, prefix string) bool {
	return profilekit.ContainsPrefix(args, prefix)
}

func insertAfterCommand(command []string, extra ...string) []string {
	if len(command) == 0 || len(extra) == 0 {
		return command
	}

	insertAt := len(command)
	for i := 1; i < len(command); i++ {
		if !strings.HasPrefix(command[i], "-") {
			insertAt = i
			break
		}
	}

	out := append([]string{}, command[:insertAt]...)
	out = append(out, extra...)
	return append(out, command[insertAt:]...)
}
