package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"szr/internal/config"
	"szr/internal/history"
)

func (a *App) runSpread(args []string) int {
	showHistory := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown spread flag %s\n", arg)
			return 2
		}
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	summary := history.Summarize(records, 8)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
		return 0
	}

	if summary.Commands == 0 {
		fmt.Println("no tracked commands yet")
		return 0
	}

	fmt.Printf("commands: %d\n", summary.Commands)
	fmt.Printf("avg savings: %.1f%%\n", summary.AveragePct)
	fmt.Printf("tokens saved: %d\n", summary.SavedTokens)
	fmt.Printf("duration p50/p95: %dms / %dms\n", summary.DurationP50MS, summary.DurationP95MS)
	fmt.Printf("bytes read/parsed/emitted: %d / %d / %d\n", summary.RawBytesRead, summary.BytesParsed, summary.BytesEmitted)
	fmt.Printf("failure rate: %.1f%% (%d/%d)\n", summary.FailureRate, summary.Failures, summary.Commands)
	fmt.Printf("fallback rate: %.1f%% (%d/%d)\n", summary.FallbackRate, summary.Fallbacks, summary.Commands)
	fmt.Printf("tee rate: %.1f%% (%d/%d)\n", summary.TeeRate, summary.TeeCount, summary.Commands)
	if len(summary.TopCommands) > 0 {
		fmt.Println("top commands:")
		for _, cmd := range summary.TopCommands {
			fmt.Printf("  %s (%d)\n", cmd.Command, cmd.Count)
		}
	}
	if len(summary.ProfileStats) > 0 {
		fmt.Println("profiles:")
		for _, stat := range summary.ProfileStats {
			fmt.Printf(
				"  %s  confidence=%s count=%d saved=%d avg=%.1f%% p50/p95=%d/%dms fail=%.1f%% fallback=%.1f%% tee=%.1f%%\n",
				stat.Name,
				stat.Confidence,
				stat.Commands,
				stat.SavedTokens,
				stat.AveragePct,
				stat.DurationP50MS,
				stat.DurationP95MS,
				stat.FailureRate,
				stat.FallbackRate,
				stat.TeeRate,
			)
		}
	}
	if len(summary.FingerprintHotspots) > 0 {
		fmt.Println("poor savings fingerprints:")
		for _, stat := range summary.FingerprintHotspots {
			fmt.Printf(
				"  %s  profile=%s count=%d avg=%.1f%% p50/p95=%d/%dms fp=%s\n",
				stat.Command,
				stat.Profile,
				stat.Commands,
				stat.AveragePct,
				stat.DurationP50MS,
				stat.DurationP95MS,
				stat.Fingerprint,
			)
		}
	}
	if len(summary.BudgetSuggestions) > 0 {
		fmt.Println("budget suggestions:")
		for _, suggestion := range summary.BudgetSuggestions {
			fmt.Printf(
				"  %s  profile=%s samples=%d %s/%s target=%d lines %d bytes %d tokens confidence=%s\n",
				suggestion.Command,
				suggestion.Profile,
				suggestion.Samples,
				suggestion.Direction,
				suggestion.Reason,
				suggestion.Suggested.MaxLines,
				suggestion.Suggested.MaxBytes,
				suggestion.Suggested.MaxTokens,
				suggestion.Confidence,
			)
		}
	}
	if showHistory {
		fmt.Println("recent:")
		for _, rec := range summary.Recent {
			fmt.Printf(
				"  %s  %s  confidence=%s  %dms  exit=%d  fallback=%t  %.1f%%  %s\n",
				rec.Timestamp.Format(time.RFC3339),
				rec.Profile,
				rec.ProfileConfidence,
				rec.DurationMS,
				rec.ExitCode,
				rec.FallbackUsed,
				rec.SavingsPct,
				rec.Command,
			)
		}
	}
	return 0
}

func (a *App) runProfiles() int {
	for _, profile := range a.engine.Profiles() {
		fmt.Printf("%s\n  %s\n", profile.Name, profile.Description)
		if profile.Confidence != "" {
			fmt.Printf("  confidence: %s\n", profile.Confidence)
		}
		if profile.StreamPreference != "" {
			fmt.Printf("  stream: %s\n", profile.StreamPreference)
		}
		if profile.Budget.MaxLines > 0 || profile.Budget.MaxBytes > 0 || profile.Budget.MaxTokens > 0 {
			fmt.Printf("  budget: lines=%d bytes=%d tokens=%d\n", profile.Budget.MaxLines, profile.Budget.MaxBytes, profile.Budget.MaxTokens)
		}
		if profile.Budget.MinFailures > 0 || profile.Budget.MinAnchors > 0 || profile.Budget.MinHints > 0 {
			fmt.Printf("  contract: failures=%d anchors=%d hints=%d\n", profile.Budget.MinFailures, profile.Budget.MinAnchors, profile.Budget.MinHints)
		}
		if profile.LatencyBudget > 0 {
			fmt.Printf("  latency budget: %s\n", profile.LatencyBudget.Round(time.Millisecond))
		}
	}
	return 0
}

func (a *App) runDoctor(cfg config.Config) int {
	fmt.Printf("version: %s\n", a.version)
	fmt.Printf("config: %s\n", a.paths.ConfigFile)
	fmt.Printf("history: %s\n", a.paths.HistoryFile)
	fmt.Printf("tee dir: %s\n", a.paths.TeeDir)
	fmt.Printf("reasoning budget mode: %s\n", cfg.ReasoningBudgetMode)
	if a.paths.ProjectRuleFile != "" {
		fmt.Printf("project rules: %s\n", a.paths.ProjectRuleFile)
	}
	for _, tool := range []string{"git", "go", "rg"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Printf("%s: missing\n", tool)
			continue
		}
		fmt.Printf("%s: %s\n", tool, path)
	}
	return 0
}
