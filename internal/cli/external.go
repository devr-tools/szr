package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"szr/internal/engine"
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
	inv := engine.Invocation{
		Command:      command,
		Display:      display,
		Cwd:          cwd,
		Verbose:      flags.verbose,
		UltraCompact: flags.ultra,
	}
	result, err := a.engine.Execute(ctx, inv, passthrough)
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

func (a *App) runExplain(flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: explain requires a command")
		return 2
	}

	cwd, _ := os.Getwd()
	profile := a.engine.Explain(engine.Invocation{
		Command:      args,
		Display:      args,
		Cwd:          cwd,
		Verbose:      flags.verbose,
		UltraCompact: flags.ultra,
	})
	fmt.Printf("profile: %s\n", profile.Name)
	fmt.Printf("about: %s\n", profile.Description)
	if profile.Confidence != "" {
		fmt.Printf("confidence: %s\n", profile.Confidence)
	}
	if profile.StreamPreference != "" {
		fmt.Printf("stream: %s\n", profile.StreamPreference)
	}
	if profile.Budget.MaxLines > 0 || profile.Budget.MaxBytes > 0 || profile.Budget.MaxTokens > 0 {
		fmt.Printf(
			"budget: lines=%d bytes=%d tokens=%d\n",
			profile.Budget.MaxLines,
			profile.Budget.MaxBytes,
			profile.Budget.MaxTokens,
		)
	}
	if profile.LatencyBudget > 0 {
		fmt.Printf("latency budget: %s\n", profile.LatencyBudget.Round(time.Millisecond))
	}
	for _, line := range profile.Explain {
		fmt.Printf("- %s\n", line)
	}
	return 0
}
