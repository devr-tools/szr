package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/szr/internal/bench"
	"github.com/devr-tools/szr/internal/installers"
)

func (a *App) runInstall(args []string) int {
	allTargets, printOnly, targets, code := parseTargetArgs("install", args)
	if code != 0 {
		return code
	}
	if allTargets && len(targets) > 0 {
		fmt.Fprintln(os.Stderr, "szr: install accepts either --all or explicit targets")
		return 2
	}
	if !allTargets && len(targets) == 0 {
		printInstallTargets()
		return 0
	}

	cwd, code := installRepoRoot(allTargets)
	if code != 0 {
		return code
	}
	plans, code := renderInstallPlans(cwd, allTargets, targets)
	if code != 0 {
		return code
	}

	return applyInstallPlans(plans, printOnly)
}

func (a *App) runUninstall(args []string) int {
	allTargets, printOnly, targets, code := parseTargetArgs("uninstall", args)
	if code != 0 {
		return code
	}
	if allTargets && len(targets) > 0 {
		fmt.Fprintln(os.Stderr, "szr: uninstall accepts either --all or explicit targets")
		return 2
	}
	if !allTargets && len(targets) == 0 {
		printUninstallTargets()
		return 0
	}

	cwd, code := installRepoRoot(allTargets)
	if code != 0 {
		return code
	}
	plans, code := renderUninstallPlans(cwd, allTargets, targets)
	if code != 0 {
		return code
	}

	return applyUninstallPlans(plans, printOnly)
}

func parseTargetArgs(verb string, args []string) (bool, bool, []installers.Target, int) {
	allTargets := false
	printOnly := false
	targets := make([]installers.Target, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--all":
			allTargets = true
		case "--print":
			printOnly = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown %s flag %s\n", verb, arg)
				return false, false, nil, 2
			}
			targets = append(targets, installers.Target(arg))
		}
	}
	return allTargets, printOnly, targets, 0
}

func installRepoRoot(allTargets bool) (string, int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return "", installPathErrorCode(allTargets)
	}
	if _, err := os.Stat(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return "", installPathErrorCode(allTargets)
	}
	return cwd, 0
}

func installPathErrorCode(allTargets bool) int {
	if allTargets {
		return 1
	}
	return 2
}

func renderInstallPlans(cwd string, allTargets bool, targets []installers.Target) ([]installers.Plan, int) {
	if allTargets {
		plans, err := installers.RenderAll(installers.Options{RepoRoot: cwd})
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return nil, 1
		}
		return plans, 0
	}

	plans := make([]installers.Plan, 0, len(targets))
	for _, target := range targets {
		plan, err := installers.Render(target, installers.Options{RepoRoot: cwd})
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return nil, 2
		}
		plans = append(plans, plan)
	}
	return plans, 0
}

func renderUninstallPlans(cwd string, allTargets bool, targets []installers.Target) ([]installers.Plan, int) {
	if allTargets {
		plans, err := installers.RenderAllUninstall(installers.Options{RepoRoot: cwd})
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return nil, 1
		}
		return plans, 0
	}

	plans := make([]installers.Plan, 0, len(targets))
	for _, target := range targets {
		plan, err := installers.RenderUninstall(target, installers.Options{RepoRoot: cwd})
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return nil, 2
		}
		plans = append(plans, plan)
	}
	return plans, 0
}

func applyInstallPlans(plans []installers.Plan, printOnly bool) int {
	for _, plan := range plans {
		if printOnly {
			printInstallPlan(plan)
			continue
		}
		if err := installers.Apply(plan); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to install %s: %v\n", plan.Target, err)
			return 1
		}
		printInstalledPlan(plan)
	}
	return 0
}

func applyUninstallPlans(plans []installers.Plan, printOnly bool) int {
	for _, plan := range plans {
		if printOnly {
			printUninstallPlan(plan)
			continue
		}
		if err := installers.Apply(plan); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to uninstall %s: %v\n", plan.Target, err)
			return 1
		}
		printUninstalledPlan(plan)
	}
	return 0
}

type benchResult struct {
	FixtureName        string   `json:"fixture_name"`
	Class              string   `json:"class"`
	ProfileName        string   `json:"profile_name"`
	CommandFingerprint string   `json:"command_fingerprint"`
	DurationMS         int64    `json:"duration_ms"`
	DurationP50US      int64    `json:"duration_p50_us"`
	DurationP95US      int64    `json:"duration_p95_us"`
	DurationMaxUS      int64    `json:"duration_max_us"`
	Samples            int      `json:"samples"`
	RawBytes           int      `json:"raw_bytes"`
	ParsedBytes        int      `json:"parsed_bytes"`
	FilteredBytes      int      `json:"filtered_bytes"`
	EmittedBytes       int      `json:"emitted_bytes"`
	SavedBytes         int      `json:"saved_bytes"`
	ByteSavingsPct     float64  `json:"byte_savings_pct"`
	RawTokens          int      `json:"raw_tokens"`
	FilteredTokens     int      `json:"filtered_tokens"`
	SavedTokens        int      `json:"saved_tokens"`
	TokenSavingsPct    float64  `json:"token_savings_pct"`
	FallbackRatePct    float64  `json:"fallback_rate_pct"`
	TeeRatePct         float64  `json:"tee_rate_pct"`
	FailureRatePct     float64  `json:"failure_rate_pct"`
	QualityScore       int      `json:"quality_score"`
	QualityIssues      []string `json:"quality_issues,omitempty"`
	ProfileConfidence  string   `json:"profile_confidence"`
	ExpectedOK         bool     `json:"expected_ok"`
	TokenSavingsOK     bool     `json:"token_savings_ok"`
	QualityOK          bool     `json:"quality_ok"`
	OK                 bool     `json:"ok"`
}

func (a *App) runBench(args []string) int {
	asJSON := false
	filters := []string{}
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown bench flag %s\n", arg)
				return 2
			}
			filters = append(filters, arg)
		}
	}

	fixtures := selectBenchFixtures(bench.MustFixtures(), filters)
	if len(fixtures) == 0 {
		fmt.Fprintln(os.Stderr, "szr: no benchmark fixtures matched")
		return 2
	}

	harness := bench.NewHarnessWithProfiles(a.engine.Profiles())
	results := make([]benchResult, 0, len(fixtures))
	for _, fixture := range fixtures {
		measurement, err := harness.Measure(fixture)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to benchmark %s: %v\n", fixture.Name, err)
			return 1
		}
		results = append(results, benchResult{
			FixtureName:        measurement.FixtureName,
			Class:              measurement.Class,
			ProfileName:        measurement.ProfileName,
			CommandFingerprint: measurement.CommandFingerprint,
			DurationMS:         measurement.Duration.Milliseconds(),
			DurationP50US:      measurement.Durations.P50.Microseconds(),
			DurationP95US:      measurement.Durations.P95.Microseconds(),
			DurationMaxUS:      measurement.Durations.Max.Microseconds(),
			Samples:            measurement.Durations.Samples,
			RawBytes:           measurement.RawBytes,
			ParsedBytes:        measurement.ParsedBytes,
			FilteredBytes:      measurement.FilteredBytes,
			EmittedBytes:       measurement.EmittedBytes,
			SavedBytes:         measurement.SavedBytes,
			ByteSavingsPct:     measurement.ByteSavingsPct,
			RawTokens:          measurement.RawTokens,
			FilteredTokens:     measurement.FilteredTokens,
			SavedTokens:        measurement.SavedTokens,
			TokenSavingsPct:    measurement.TokenSavingsPct,
			FallbackRatePct:    measurement.FallbackRate,
			TeeRatePct:         measurement.TeeRate,
			FailureRatePct:     measurement.FailureRate,
			QualityScore:       measurement.Quality.Score,
			QualityIssues:      append([]string(nil), measurement.Quality.Issues...),
			ProfileConfidence:  measurement.Quality.ProfileConfidence,
			ExpectedOK:         measurement.Expectation.ContainsOK,
			TokenSavingsOK:     measurement.Expectation.TokenSavingsOK,
			QualityOK:          measurement.Expectation.QualityOK,
			OK:                 measurement.Expectation.OK,
		})
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(results)
		return 0
	}

	exitCode := 0
	for _, result := range results {
		fmt.Printf(
			"%s profile=%s tokens=%.1f%% bytes=%.1f%% parsed=%dB dur_p50=%dus dur_p95=%dus quality=%d fallback=%.0f%% ok=%t\n",
			result.FixtureName,
			result.ProfileName,
			result.TokenSavingsPct,
			result.ByteSavingsPct,
			result.ParsedBytes,
			result.DurationP50US,
			result.DurationP95US,
			result.QualityScore,
			result.FallbackRatePct,
			result.OK,
		)
		if !result.OK {
			exitCode = 1
		}
	}
	return exitCode
}

func selectBenchFixtures(fixtures []bench.Fixture, names []string) []bench.Fixture {
	if len(names) == 0 {
		return fixtures
	}

	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}

	filtered := make([]bench.Fixture, 0, len(fixtures))
	for _, fixture := range fixtures {
		if _, ok := allowed[fixture.Name]; ok {
			filtered = append(filtered, fixture)
		}
	}
	return filtered
}

func printInstallPlan(plan installers.Plan) {
	fmt.Printf("plan: %s\n", plan.Target)
	for _, file := range plan.Files {
		fmt.Printf("  %s  %s\n", relativeToRepo(plan.Paths.RepoRoot, file.Path), file.Description)
	}
	if len(plan.ManualSteps) > 0 {
		fmt.Println("  manual steps:")
		for _, step := range plan.ManualSteps {
			fmt.Printf("    - %s\n", step)
		}
	}
}

func printUninstallPlan(plan installers.Plan) {
	fmt.Printf("plan: uninstall %s\n", plan.Target)
	for _, file := range plan.Files {
		fmt.Printf("  %s  %s\n", relativeToRepo(plan.Paths.RepoRoot, file.Path), file.Description)
	}
	if len(plan.ManualSteps) > 0 {
		fmt.Println("  manual steps:")
		for _, step := range plan.ManualSteps {
			fmt.Printf("    - %s\n", step)
		}
	}
}

func printInstalledPlan(plan installers.Plan) {
	fmt.Printf("installed %s\n", plan.Target)
	for _, file := range plan.Files {
		fmt.Printf("  %s\n", relativeToRepo(plan.Paths.RepoRoot, file.Path))
	}
	for _, step := range plan.ManualSteps {
		fmt.Printf("  next: %s\n", step)
	}
}

func printUninstalledPlan(plan installers.Plan) {
	fmt.Printf("uninstalled %s\n", plan.Target)
	for _, file := range plan.Files {
		fmt.Printf("  %s\n", relativeToRepo(plan.Paths.RepoRoot, file.Path))
	}
	for _, step := range plan.ManualSteps {
		fmt.Printf("  next: %s\n", step)
	}
}

func printInstallTargets() {
	targets := installers.Targets()
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, string(target))
	}
	fmt.Println("available install targets:")
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()
	fmt.Printf("use: szr install <%s>\n", strings.Join(names, "|"))
	fmt.Println("or:  szr install --all")
}

func printUninstallTargets() {
	targets := installers.Targets()
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, string(target))
	}
	fmt.Println("available uninstall targets:")
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()
	fmt.Printf("use: szr uninstall <%s>\n", strings.Join(names, "|"))
	fmt.Println("or:  szr uninstall --all")
}

func relativeToRepo(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return "./" + strings.TrimPrefix(rel, "./")
}
