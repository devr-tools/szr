package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profiles"
)

func RunReplay(rt Runtime, args []string) int {
	opts, code := parseReplayArgs(rt, args)
	if code != 0 {
		return code
	}
	raw, entry, foundEntry, err := readReplayTarget(rt, opts.target)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	commandText, cwd, exitCode := resolveReplayContext(opts, entry, foundEntry)
	if commandText == "" && opts.profileName == "" {
		fmt.Fprintln(rt.Stderr, "szr: replay requires --command or --profile when replaying a plain file")
		return 2
	}

	cfg := rt.Config
	if opts.maxLines > 0 {
		cfg.MaxPreviewLines = opts.maxLines
	}
	eng := engine.New(cfg, rt.Paths, rt.History, profiles.Builtins(cfg.MaxPreviewLines))

	inv := engine.Invocation{
		Command:             strings.Fields(commandText),
		Display:             strings.Fields(commandText),
		Cwd:                 cwd,
		Verbose:             rt.Verbose,
		UltraCompact:        rt.UltraCompact,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
		Advanced:            cfg.Advanced,
	}
	effectiveInv, _ := eng.ExplainPreferences(inv)
	profile, err := selectedProfile(eng, inv, opts.profileName)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 2
	}

	execResult := executionForReplay(profile, raw, exitCode)
	rendered := engine.RenderExecution(profile, effectiveInv, execResult, cfg.MaxPreviewLines, false)
	output := buildReplayOutput(commandText, effectiveInv.Command, profile, cfg.MaxPreviewLines, execResult.ExitCode, rendered)
	if opts.asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
		return 0
	}

	if commandText != "" {
		fmt.Fprintf(rt.Stdout, "command: %s\n", commandText)
	}
	if len(effectiveInv.Command) > 0 {
		fmt.Fprintf(rt.Stdout, "effective command: %s\n", strings.Join(effectiveInv.Command, " "))
	}
	fmt.Fprintf(rt.Stdout, "profile: %s\n", profile.Name)
	if profile.Confidence != "" {
		fmt.Fprintf(rt.Stdout, "confidence: %s\n", profile.Confidence)
	}
	fmt.Fprintf(rt.Stdout, "exit: %d\n", execResult.ExitCode)
	fmt.Fprintf(rt.Stdout, "fallback: %t\n", output.FallbackUsed)
	fmt.Fprintf(
		rt.Stdout,
		"tokens: raw=%d out=%d saved=%d (%.1f%%)\n",
		output.RawTokens,
		output.FilteredTokens,
		output.SavedTokens,
		output.SavingsPct,
	)
	fmt.Fprintf(rt.Stdout, "budget: lines=%d bytes=%d tokens=%d\n", output.Budget.MaxLines, output.Budget.MaxBytes, output.Budget.MaxTokens)
	fmt.Fprintln(rt.Stdout, "rendered:")
	fmt.Fprintln(rt.Stdout, output.Display)
	return 0
}

func RunCompare(ctx context.Context, rt Runtime, args []string) int {
	asJSON := false
	maxLines := 0
	command := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--max-lines":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: compare requires a value after --max-lines")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(rt.Stderr, "szr: invalid compare max lines %q\n", args[i])
				return 2
			}
			maxLines = value
		default:
			if len(command) == 0 && strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(rt.Stderr, "szr: unknown compare flag %s\n", args[i])
				return 2
			}
			command = append(command, args[i])
		}
	}
	if len(command) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: compare requires a command")
		return 2
	}

	cfg := rt.Config
	if maxLines > 0 {
		cfg.MaxPreviewLines = maxLines
	}
	eng := engine.New(cfg, rt.Paths, rt.History, profiles.Builtins(cfg.MaxPreviewLines))

	cwd, _ := os.Getwd()
	inv := engine.Invocation{
		Command:             append([]string(nil), command...),
		Display:             append([]string(nil), command...),
		Cwd:                 cwd,
		Verbose:             rt.Verbose,
		UltraCompact:        rt.UltraCompact,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
		Advanced:            cfg.Advanced,
	}
	effectiveInv, _ := eng.ExplainPreferences(inv)
	profile := eng.Explain(inv)
	effectiveCommand := preparedCommand(profile, effectiveInv)
	resolvedCommand, err := resolveExecutableCommand(effectiveCommand)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 2
	}

	execResult, duration, err := captureExecution(ctx, resolvedCommand, cwd)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	rendered := engine.RenderExecution(profile, effectiveInv, execResult, cfg.MaxPreviewLines, false)
	rawPreview := filters.CompactLines(rendered.RawCombined, cfg.MaxPreviewLines)
	out := compareOutput{
		Command:           strings.Join(command, " "),
		EffectiveCommand:  strings.Join(effectiveCommand, " "),
		Profile:           profile.Name,
		ProfileConfidence: profile.Confidence,
		ExitCode:          execResult.ExitCode,
		DurationMS:        duration.Milliseconds(),
		FallbackUsed:      rendered.FallbackUsed,
		RawTokens:         rendered.RawTokens,
		FilteredTokens:    rendered.FilteredTokens,
		SavedTokens:       rendered.RawTokens - rendered.FilteredTokens,
		SavingsPct:        savingsPct(rendered.RawTokens, rendered.FilteredTokens),
		BytesParsed:       rendered.BytesParsed,
		BytesEmitted:      rendered.BytesEmitted,
		Budget:            engine.ResolveBudget(profile, effectiveInv, cfg.MaxPreviewLines),
		RawPreview:        rawPreview,
		ReducedPreview:    rendered.Text,
	}
	if asJSON {
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
	}

	fmt.Fprintf(rt.Stdout, "command: %s\n", out.Command)
	fmt.Fprintf(rt.Stdout, "effective command: %s\n", out.EffectiveCommand)
	fmt.Fprintf(rt.Stdout, "profile: %s\n", out.Profile)
	if out.ProfileConfidence != "" {
		fmt.Fprintf(rt.Stdout, "confidence: %s\n", out.ProfileConfidence)
	}
	fmt.Fprintf(rt.Stdout, "duration: %dms\n", out.DurationMS)
	fmt.Fprintf(rt.Stdout, "exit: %d\n", out.ExitCode)
	fmt.Fprintf(rt.Stdout, "fallback: %t\n", out.FallbackUsed)
	fmt.Fprintf(
		rt.Stdout,
		"tokens: raw=%d out=%d saved=%d (%.1f%%)\n",
		out.RawTokens,
		out.FilteredTokens,
		out.SavedTokens,
		out.SavingsPct,
	)
	fmt.Fprintf(rt.Stdout, "budget: lines=%d bytes=%d tokens=%d\n", out.Budget.MaxLines, out.Budget.MaxBytes, out.Budget.MaxTokens)
	fmt.Fprintln(rt.Stdout, "raw preview:")
	fmt.Fprintln(rt.Stdout, out.RawPreview)
	fmt.Fprintln(rt.Stdout, "reduced preview:")
	fmt.Fprintln(rt.Stdout, out.ReducedPreview)
	return 0
}
