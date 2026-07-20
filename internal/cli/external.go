package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/diagnostics"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
)

func (a *App) runExternal(ctx context.Context, flags globalFlags, name string, args []string, passthrough bool) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "szr: missing command for %s\n", name)
		return 2
	}

	command := args
	display := args
	if name != "run" && name != "proxy" && name != "test" && name != "summary" {
		command = append([]string{name}, args...)
		display = append([]string{name}, args...)
	} else if name == "test" || name == "summary" {
		display = append([]string{name}, args...)
	}

	cwd, _ := os.Getwd()
	cfg := a.configForFlags(flags)
	inv := engine.Invocation{
		Command:             command,
		Display:             display,
		Cwd:                 cwd,
		Verbose:             flags.verbose,
		UltraCompact:        flags.ultra,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
		Advanced:            cfg.Advanced,
	}
	runID := diagnostics.NewRunID()
	result, err := a.engineForFlags(flags).ExecuteStreaming(ctx, inv, passthrough, func(partial engine.PartialResult) {
		a.appendProgressEvent(runID, partial)
	})
	a.appendFinalEvent(runID, result, passthrough, err)
	if flags.verbose >= 2 {
		fmt.Fprintf(
			os.Stderr,
			"[szr] profile=%s confidence=%s duration=%s exit=%d fallback=%t bypass=%q latency_warn=%t bytes=%d/%d/%d\n",
			result.ProfileName,
			result.ProfileConfidence,
			result.Duration.Round(time.Millisecond),
			result.ExitCode,
			result.FallbackUsed,
			result.BypassReason,
			result.LatencyWarning,
			result.RawBytesRead,
			result.BytesParsed,
			result.BytesEmitted,
		)
	}
	if flags.verbose >= 3 && result.RawCombined != "" {
		fmt.Fprintf(os.Stderr, "[szr] raw:\n%s\n", result.RawCombined)
	}
	if result.Display != "" {
		fmt.Println(result.Display)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	return result.ExitCode
}

func (a *App) appendProgressEvent(runID string, partial engine.PartialResult) {
	if a.events == nil || partial.Final {
		return
	}
	_ = a.events.Append(diagnostics.Event{
		Version:           diagnostics.SchemaVersion,
		Type:              diagnostics.EventRunProgress,
		RunID:             runID,
		Timestamp:         time.Now().UTC(),
		Profile:           partial.ProfileName,
		ProfileConfidence: partial.ProfileConfidence,
		BytesParsed:       partial.BytesParsed,
	})
}

//nolint:maintidx // Event construction mirrors the allowlisted schema field-for-field.
func (a *App) appendFinalEvent(runID string, result engine.Result, passthrough bool, runErr error) {
	if a.events == nil {
		return
	}
	exitClass := "success"
	if runErr != nil {
		exitClass = "error"
	} else if result.ExitCode != 0 {
		exitClass = "failure"
	}
	_ = a.events.Append(diagnostics.Event{
		Version:           diagnostics.SchemaVersion,
		Type:              diagnostics.EventRunFinal,
		RunID:             runID,
		Timestamp:         time.Now().UTC(),
		Profile:           result.ProfileName,
		ProfileConfidence: result.ProfileConfidence,
		BytesParsed:       result.BytesParsed,
		RawBytesRead:      result.RawBytesRead,
		BytesEmitted:      result.BytesEmitted,
		RawTokensEst:      result.RawTokens,
		EmittedTokensEst:  result.FilteredTokens,
		SavedTokensEst:    result.SavedTokens,
		DurationMS:        result.Duration.Milliseconds(),
		ExitClass:         exitClass,
		FallbackUsed:      result.FallbackUsed,
		Passthrough:       passthrough,
		VerifierRepairs:   result.VerifierRepairs,
		VerifierSkipped:   result.VerifierSkipped,
		DedupUsed:         result.DedupRef != "",
		DeltaUsed:         result.DeltaRef != "",
	})
}

func (a *App) runExplain(flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: explain requires a command")
		return 2
	}

	cfg := a.configForFlags(flags)
	cwd, _ := os.Getwd()
	inv := engine.Invocation{
		Command:             args,
		Display:             args,
		Cwd:                 cwd,
		Verbose:             flags.verbose,
		UltraCompact:        flags.ultra,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
		Advanced:            cfg.Advanced,
	}
	effectiveInv, preferences := a.engineForFlags(flags).ExplainPreferences(inv)
	profile := a.engineForFlags(flags).Explain(inv)
	resolvedBudget, budgetAdaptation := a.engineForFlags(flags).ExplainBudget(inv)
	decisions := a.engineForFlags(flags).ExplainDecisions(inv)
	printExplainProfile(profile, a.paths.ProjectRuleFile, cfg.ReasoningBudgetMode)
	printExplainEffectiveCommand(effectiveInv.Command, preferences)
	printExplainBudget(resolvedBudget)
	printExplainBudgetAdaptation(budgetAdaptation)
	printExplainLatency(profile)
	printExplainSuggestion(a.findBudgetSuggestion(args))
	printExplainPreferences(preferences, a.paths.ProjectRuleFile)
	printExplainDecisions(decisions, a.paths.ProjectRuleFile)
	for _, line := range profile.Explain {
		fmt.Printf("- %s\n", line)
	}
	return 0
}

func printExplainProfile(profile engine.Profile, projectRuleFile string, reasoningBudgetMode string) {
	fmt.Printf("profile: %s\n", profile.Name)
	fmt.Printf("source: %s\n", describeProfileSource(profile.Source, projectRuleFile))
	fmt.Printf("about: %s\n", profile.Description)
	fmt.Printf("reasoning budget mode: %s\n", reasoningBudgetMode)
	if profile.Confidence != "" {
		fmt.Printf("confidence: %s\n", profile.Confidence)
	}
	if profile.StreamPreference != "" {
		fmt.Printf("stream: %s\n", profile.StreamPreference)
	}
}

func printExplainEffectiveCommand(command []string, preferences []engine.PreferenceDecision) {
	if len(preferences) > 0 {
		fmt.Printf("effective command: %s\n", strings.Join(command, " "))
	}
}

func printExplainBudget(resolvedBudget engine.OutputBudget) {
	if resolvedBudget.MaxLines > 0 || resolvedBudget.MaxBytes > 0 || resolvedBudget.MaxTokens > 0 {
		fmt.Printf(
			"budget: lines=%d bytes=%d tokens=%d\n",
			resolvedBudget.MaxLines,
			resolvedBudget.MaxBytes,
			resolvedBudget.MaxTokens,
		)
	}
	if resolvedBudget.MinFailures > 0 || resolvedBudget.MinAnchors > 0 || resolvedBudget.MinHints > 0 {
		fmt.Printf("contract: failures=%d anchors=%d hints=%d\n", resolvedBudget.MinFailures, resolvedBudget.MinAnchors, resolvedBudget.MinHints)
	}
}

func printExplainBudgetAdaptation(adaptation *engine.BudgetAdaptation) {
	if adaptation == nil {
		return
	}
	fmt.Printf(
		"budget adaptation: %s %s confidence=%s samples=%d source=%s\n",
		adaptation.Direction,
		adaptation.Reason,
		adaptation.Confidence,
		adaptation.Samples,
		adaptation.Source,
	)
}

func printExplainLatency(profile engine.Profile) {
	if profile.LatencyBudget > 0 {
		fmt.Printf("latency budget: %s\n", profile.LatencyBudget.Round(time.Millisecond))
	}
}

func printExplainSuggestion(suggestion *history.BudgetSuggestion) {
	if suggestion != nil {
		fmt.Printf(
			"history suggestion: %s %s lines=%d bytes=%d tokens=%d confidence=%s samples=%d\n",
			suggestion.Direction,
			suggestion.Reason,
			suggestion.Suggested.MaxLines,
			suggestion.Suggested.MaxBytes,
			suggestion.Suggested.MaxTokens,
			suggestion.Confidence,
			suggestion.Samples,
		)
	}
}

func printExplainPreferences(preferences []engine.PreferenceDecision, projectRuleFile string) {
	if len(preferences) > 0 {
		fmt.Println("applied preferences:")
		for _, preference := range preferences {
			label := "satisfied"
			if preference.Applied {
				label = "applied"
			}
			fmt.Printf("  %s  %s  %s\n", label, describeProfileSource(preference.Source, projectRuleFile), preference.Name)
		}
	}
}

func printExplainDecisions(decisions []engine.ExplainDecision, projectRuleFile string) {
	if len(decisions) > 0 {
		fmt.Println("matched decisions:")
		for _, decision := range decisions {
			label := "also matches"
			if decision.Selected {
				label = "selected"
			}
			fmt.Printf("  %s  %s  %s\n", label, describeProfileSource(decision.Source, projectRuleFile), decision.Name)
		}
	}
}

func (a *App) findBudgetSuggestion(command []string) *history.BudgetSuggestion {
	if a.history == nil {
		return nil
	}
	fingerprint := history.Fingerprint(strings.Join(command, " "))
	suggestion, err := a.history.FindBudgetSuggestion(fingerprint, history.BudgetSuggestionOptions{Limit: 16})
	if err != nil {
		return nil
	}
	return suggestion
}

func describeProfileSource(source string, projectRuleFile string) string {
	switch source {
	case engine.SourceProject:
		if projectRuleFile != "" {
			return source + " (" + projectRuleFile + ")"
		}
		return source
	case engine.SourcePreference:
		if projectRuleFile != "" {
			return source + " (" + projectRuleFile + ")"
		}
		return source
	case "":
		return engine.SourceBuiltin
	default:
		return source
	}
}
