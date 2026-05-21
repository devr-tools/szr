package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"szr/internal/bench"
	"szr/internal/installers"
)

func (a *App) runInstall(args []string) int {
	var (
		allTargets bool
		printOnly  bool
		targets    []installers.Target
	)

	for _, arg := range args {
		switch arg {
		case "--all":
			allTargets = true
		case "--print":
			printOnly = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown install flag %s\n", arg)
				return 2
			}
			targets = append(targets, installers.Target(arg))
		}
	}

	if allTargets && len(targets) > 0 {
		fmt.Fprintln(os.Stderr, "szr: install accepts either --all or explicit targets")
		return 2
	}
	if !allTargets && len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "szr: install requires a target or --all")
		return 2
	}

	cwd, _ := os.Getwd()

	plans := make([]installers.Plan, 0, len(targets))
	var err error
	if allTargets {
		plans, err = installers.RenderAll(installers.Options{RepoRoot: cwd})
	} else {
		for _, target := range targets {
			plan, renderErr := installers.Render(target, installers.Options{RepoRoot: cwd})
			if renderErr != nil {
				fmt.Fprintf(os.Stderr, "szr: %v\n", renderErr)
				return 2
			}
			plans = append(plans, plan)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}

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

type benchResult struct {
	FixtureName     string  `json:"fixture_name"`
	Class           string  `json:"class"`
	ProfileName     string  `json:"profile_name"`
	DurationMS      int64   `json:"duration_ms"`
	RawBytes        int     `json:"raw_bytes"`
	FilteredBytes   int     `json:"filtered_bytes"`
	SavedBytes      int     `json:"saved_bytes"`
	ByteSavingsPct  float64 `json:"byte_savings_pct"`
	RawTokens       int     `json:"raw_tokens"`
	FilteredTokens  int     `json:"filtered_tokens"`
	SavedTokens     int     `json:"saved_tokens"`
	TokenSavingsPct float64 `json:"token_savings_pct"`
	ExpectedOK      bool    `json:"expected_ok"`
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
			FixtureName:     measurement.FixtureName,
			Class:           measurement.Class,
			ProfileName:     measurement.ProfileName,
			DurationMS:      measurement.Duration.Milliseconds(),
			RawBytes:        measurement.RawBytes,
			FilteredBytes:   measurement.FilteredBytes,
			SavedBytes:      measurement.SavedBytes,
			ByteSavingsPct:  measurement.ByteSavingsPct,
			RawTokens:       measurement.RawTokens,
			FilteredTokens:  measurement.FilteredTokens,
			SavedTokens:     measurement.SavedTokens,
			TokenSavingsPct: measurement.TokenSavingsPct,
			ExpectedOK:      benchExpectationOK(measurement.Rendered, fixture.ExpectedContains),
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
			"%s profile=%s tokens=%.1f%% bytes=%.1f%% dur=%dms ok=%t\n",
			result.FixtureName,
			result.ProfileName,
			result.TokenSavingsPct,
			result.ByteSavingsPct,
			result.DurationMS,
			result.ExpectedOK,
		)
		if !result.ExpectedOK {
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

func benchExpectationOK(rendered string, expected []string) bool {
	for _, fragment := range expected {
		if !strings.Contains(rendered, fragment) {
			return false
		}
	}
	return true
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

func printInstalledPlan(plan installers.Plan) {
	fmt.Printf("installed %s\n", plan.Target)
	for _, file := range plan.Files {
		fmt.Printf("  %s\n", relativeToRepo(plan.Paths.RepoRoot, file.Path))
	}
	for _, step := range plan.ManualSteps {
		fmt.Printf("  next: %s\n", step)
	}
}

func relativeToRepo(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return "./" + strings.TrimPrefix(rel, "./")
}
