package gradle

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	gradlefilter "github.com/devr-tools/szr/internal/filters/gradle"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{gradleBuildProfile(maxLines)}
}

var gradleBuildExplain = []string{
	"Matches `gradle` and `./gradlew` invocations running build, test, or check tasks ahead of the generic build-system profile.",
	"On failure keeps failed task headers, compiler diagnostics with file:line anchors, and failing test names; on success compacts to the BUILD SUCCESSFUL line plus task counts.",
	"Handles both rich and `--console=plain` output and requests plain console when aggressive rewrites are enabled.",
}

func matchGradleBuild(inv engine.Invocation) bool {
	return isGradleBuildCommand(inv.Display)
}

func isGradleBuildCommand(args []string) bool {
	if len(args) < 2 || !isGradleEntrypoint(args[0]) {
		return false
	}
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		switch arg[strings.LastIndex(arg, ":")+1:] {
		case "build", "test", "check":
			return true
		}
	}
	return false
}

func isGradleEntrypoint(head string) bool {
	return head == "gradle" || head == "gradlew" || strings.HasSuffix(head, "/gradlew")
}

func prepareGradleBuild(inv engine.Invocation) []string {
	if !inv.Advanced.AggressivePrepareRewrites {
		return inv.Command
	}
	out := append([]string{}, inv.Command...)
	if !profilekit.ContainsPrefix(out[1:], "--console=") {
		out = append(out, "--console=plain")
	}
	return out
}

func renderGradleBuild(maxLines int) func(engine.Invocation, engine.Execution) string {
	return func(inv engine.Invocation, exec engine.Execution) string {
		return gradlefilter.SummarizeGradleUnderContract(exec.Stdout+"\n"+exec.Stderr, maxLines, inv.Advanced.CompressionContract)
	}
}

func streamRenderGradleBuild(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
	// Raw (unstripped) buffer keeps the self-cap prediction aligned with the
	// raw stream the compression contract budgets against.
	contract := inv.Advanced.CompressionContract
	return shared.NewBufferedRawTextReducer(
		true,
		true,
		func(input string) string {
			return gradlefilter.SummarizeGradleUnderContract(input, budget.MaxLines, contract)
		},
		func(input string) (string, string, bool) {
			return gradlefilter.GradleRecoveryInfoUnderContract(input, budget.MaxLines, contract)
		},
	)
}

func gradleBuildProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:             "gradle-build",
		Description:      "Focuses gradle build, test, and check output on failed tasks, diagnostics, and the build result line.",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 12)),
		LatencyBudget:    profilekit.LatencyBudget(30),
		Match:            matchGradleBuild,
		Prepare:          prepareGradleBuild,
		Render:           renderGradleBuild(maxLines),
		StreamRender:     streamRenderGradleBuild,
		ParseBytes:       profilekit.ParseCombined,
		Explain:          gradleBuildExplain,
	}
}
