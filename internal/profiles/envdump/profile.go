package envdump

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	envfilter "github.com/devr-tools/szr/internal/filters/envdump"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{envPrintProfile(maxLines)}
}

var envPrintExplain = []string{
	"Matches only a bare `env` with no flags, assignments, or wrapped command, so `env KEY=VAL cmd` stays untouched.",
	"Keeps PATH and common diagnostic variables readable, groups the rest by name prefix, and redacts values of secret-looking names.",
}

func renderEnvPrint(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(_ engine.Invocation, exec engine.Execution) string {
		return envfilter.SummarizeEnvDump(exec.Stdout, maxLines)
	}
}

func streamRenderEnvPrint(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	return shared.NewBufferedTextReducerWithRecovery(
		true,
		false,
		func(input string) string {
			return envfilter.SummarizeEnvDump(input, budget.MaxLines)
		},
		func(input string) (string, string, bool) {
			return envfilter.EnvDumpRecoveryInfo(input, budget.MaxLines)
		},
	)
}

func envPrintProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "env-print",
		Description:      "Compacts bare `env` dumps into grouped variables with secret-looking values redacted.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchEnvPrint,
		Render:           renderEnvPrint(maxLines),
		StreamRender:     streamRenderEnvPrint,
		ParseBytes:       profilekit.ParseStdout,
		Explain:          envPrintExplain,
	}
}

// matchEnvPrint claims only a bare `env`: any flag, assignment, or trailing
// argument means env is configuring or wrapping another program.
func matchEnvPrint(inv engine.Invocation) bool {
	return len(inv.Display) == 1 && inv.Display[0] == "env"
}
