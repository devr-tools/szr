package pulumi

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	pulumifilter "github.com/devr-tools/szr/internal/filters/pulumi"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{pulumiDiffProfile(maxLines)}
}

var pulumiDiffExplain = []string{
	"Matches `pulumi preview`, `up`, `destroy`, and `refresh` invocations.",
	"Compacts the resource-diff table to change rows only, keeping diagnostics in full plus the resource counts and duration summary.",
}

func matchPulumiDiff(inv engine.Invocation) bool {
	return isPulumiDiffCommand(inv.Display)
}

func isPulumiDiffCommand(args []string) bool {
	if len(args) < 2 || args[0] != "pulumi" {
		return false
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			if _, ok := pulumiValueFlags[arg]; ok && !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		switch arg {
		case "preview", "up", "destroy", "refresh":
			return true
		default:
			return false
		}
	}
	return false
}

var pulumiValueFlags = map[string]struct{}{
	"-s":            {},
	"--stack":       {},
	"-C":            {},
	"--cwd":         {},
	"--config":      {},
	"--config-file": {},
	"--color":       {},
}

func renderPulumiDiff(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(inv engine.Invocation, exec engine.Execution) string {
		return pulumifilter.SummarizePulumiUnderContract(exec.Stdout+"\n"+exec.Stderr, maxLines, inv.Advanced.CompressionContract)
	}
}

func streamRenderPulumiDiff(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	// Raw (unstripped) buffer keeps the self-cap prediction aligned with the
	// raw stream the compression contract budgets against.
	contract := inv.Advanced.CompressionContract
	return shared.NewBufferedRawTextReducer(
		true,
		true,
		func(input string) string {
			return pulumifilter.SummarizePulumiUnderContract(input, budget.MaxLines, contract)
		},
		func(input string) (string, string, bool) {
			return pulumifilter.PulumiRecoveryInfoUnderContract(input, budget.MaxLines, contract)
		},
	)
}

func pulumiDiffProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "pulumi-diff",
		Description:      "Compacts pulumi preview/up/destroy/refresh output to changed resources, diagnostics, and the summary counts.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match:            matchPulumiDiff,
		Render:           renderPulumiDiff(maxLines),
		StreamRender:     streamRenderPulumiDiff,
		ParseBytes:       profilekit.ParseCombined,
		Explain:          pulumiDiffExplain,
	}
}
