package jsonquery

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	jsonqueryfilter "github.com/devr-tools/szr/internal/filters/jsonquery"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "json-query",
			Description:      "Normalizes JSON query and explicit JSON display commands into bounded structured output.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return isJSONQueryCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareJSONQuery(inv)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return jsonqueryfilter.SummarizeQueryOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewBufferedTextReducerWithRecovery(
					true,
					true,
					func(input string) string {
						return jsonqueryfilter.SummarizeQueryOutput(input, "", budget.MaxLines)
					},
					func(input string) (string, string, bool) {
						return jsonqueryfilter.QueryOutputRecoveryInfo(input, "", budget.MaxLines)
					},
				)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `jq`, query-style `yq`, and explicit raw JSON display commands such as `cat file.json` and `python -m json.tool file.json`.",
				"Preserves structured JSON values with stable indentation and falls back to compact error output when the command does not emit valid JSON.",
			},
		},
	}
}

func isJSONQueryCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "jq":
		return true
	case "yq":
		return isYQQueryCommand(args)
	case "cat":
		return isSingleJSONFileRead(args)
	case "python", "python3":
		return isPythonJSONTool(args)
	default:
		return false
	}
}

func prepareJSONQuery(inv engine.Invocation) []string {
	if len(inv.Command) == 0 || !inv.Advanced.AggressivePrepareRewrites {
		return inv.Command
	}

	switch inv.Command[0] {
	case "jq":
		return prepareJQ(inv.Command)
	case "yq":
		return prepareYQ(inv.Command)
	default:
		return inv.Command
	}
}

func prepareJQ(command []string) []string {
	if len(command) == 0 {
		return command
	}
	if profilekit.ContainsAny(command[1:], "-M", "-C", "--monochrome-output", "--color-output") {
		return command
	}
	if profilekit.ContainsPrefix(command[1:], "--color-output=") {
		return command
	}
	out := append([]string{}, command...)
	return append(out, "-M")
}

func prepareYQ(command []string) []string {
	if len(command) == 0 {
		return command
	}
	args := command[1:]
	if profilekit.ContainsAny(args, "-j", "--tojson") ||
		profilekit.ContainsPrefix(args, "-o=") ||
		profilekit.ContainsPrefix(args, "--output-format=") ||
		profilekit.ContainsAny(args, "-o", "--output-format") {
		return command
	}
	out := append([]string{}, command...)
	return append(out, "-o=json")
}

func isYQQueryCommand(args []string) bool {
	if len(args) == 1 {
		return true
	}
	seenPositional := false
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !seenPositional && looksLikeYQExpression(arg) {
			return true
		}
		seenPositional = true
		switch arg {
		case "e", "eval", "r", "read":
			return true
		default:
			return false
		}
	}
	return true
}

func looksLikeYQExpression(arg string) bool {
	return strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "[") || strings.HasPrefix(arg, "{")
}

func isSingleJSONFileRead(args []string) bool {
	return len(args) == 2 && hasJSONLikeExtension(args[1])
}

func isPythonJSONTool(args []string) bool {
	return len(args) >= 4 && args[1] == "-m" && args[2] == "json.tool" && hasJSONLikeExtension(args[len(args)-1])
}

func hasJSONLikeExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".jsonl", ".ndjson":
		return true
	default:
		return false
	}
}
