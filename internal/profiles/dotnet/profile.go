package dotnet

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	dotnetfilter "github.com/devr-tools/szr/internal/filters/dotnet"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{
		testProfile(maxLines),
		buildProfile(maxLines),
	}
}

func testProfile(maxLines int) engine.Profile {
	return profilekit.WithSummary(engine.Profile{
		Name:        "dotnet-test",
		Description: "Summarizes `dotnet test` output around failed test names, assertion messages, and stack anchors.",
		Confidence:  engine.ConfidenceHigh,
		Match:       matchDotnetTest,
		Prepare:     prepareDotnet,
		Explain: []string{
			"Recognizes `dotnet test` and trims the MSBuild banner with `--nologo` when the command did not already pass it.",
			"Keeps every failed test name with its assertion message and first stack anchor while collapsing discovery and pass chatter.",
		},
	}, summaryConfig(maxLines, 12, 35, dotnetfilter.SummarizeDotnetTest, dotnetfilter.DotnetTestRecoveryInfo))
}

func buildProfile(maxLines int) engine.Profile {
	return profilekit.WithSummary(engine.Profile{
		Name:        "dotnet-build",
		Description: "Summarizes `dotnet build` and MSBuild output around coded compiler diagnostics.",
		Confidence:  engine.ConfidenceHigh,
		Match:       matchDotnetBuild,
		Prepare:     prepareDotnet,
		Explain: []string{
			"Matches `dotnet build`, `dotnet publish`, `dotnet pack`, and direct `msbuild` invocations.",
			"Keeps every `error CSNNNN`-style diagnostic with its file(line,col) anchor plus the Build FAILED summary, dropping restore and progress noise.",
		},
	}, summaryConfig(maxLines, 10, 30, dotnetfilter.SummarizeDotnetBuild, dotnetfilter.DotnetBuildRecoveryInfo))
}

// summaryConfig wires a combined stdout+stderr buffered reducer with
// recovery info into a profilekit summary configuration.
func summaryConfig(
	maxLines, floor, latencyMS int,
	summarize func(string, int) string,
	recovery func(string, int) (string, string, bool),
) profilekit.SummaryConfig {
	limit := profilekit.AtLeast(maxLines, floor)
	return profilekit.SummaryConfig{
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(limit),
		LatencyBudget:    latencyMS,
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return summarize(exec.Stdout+"\n"+exec.Stderr, limit)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(true, true, func(input string) string {
				return summarize(input, budget.MaxLines)
			}, func(input string) (string, string, bool) {
				return recovery(input, budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseCombined,
	}
}

func matchDotnetTest(inv engine.Invocation) bool {
	return profilekit.HasCommand(inv.Command, "dotnet", "test") || profilekit.HasCommand(inv.Display, "dotnet", "test")
}

func matchDotnetBuild(inv engine.Invocation) bool {
	return isDotnetBuildCommand(inv.Command) || isDotnetBuildCommand(inv.Display)
}

func prepareDotnet(inv engine.Invocation) []string {
	return ensureDotnetNologo(inv.Command)
}

func isDotnetBuildCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] == "msbuild" {
		return true
	}
	if args[0] != "dotnet" || len(args) < 2 {
		return false
	}
	switch args[1] {
	case "build", "publish", "pack", "msbuild":
		return true
	default:
		return false
	}
}

// ensureDotnetNologo appends --nologo for dotnet invocations (before any
// `--` application-argument separator) unless the user already chose it.
func ensureDotnetNologo(command []string) []string {
	if len(command) == 0 || command[0] != "dotnet" || profilekit.ContainsAny(command[1:], "--nologo") {
		return command
	}
	insertAt := len(command)
	for i, arg := range command {
		if arg == "--" {
			insertAt = i
			break
		}
	}
	out := append([]string{}, command[:insertAt]...)
	out = append(out, "--nologo")
	return append(out, command[insertAt:]...)
}
