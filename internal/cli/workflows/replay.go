package workflows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/internal/teeindex"
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
	}
	effectiveInv, _ := eng.ExplainPreferences(inv)
	profile := eng.Explain(inv)
	effectiveCommand := preparedCommand(profile, effectiveInv)

	execResult, duration, err := captureExecution(ctx, effectiveCommand, cwd)
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

func parseReplayArgs(rt Runtime, args []string) (replayOptions, int) {
	opts := replayOptions{}
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			nextIndex, code := applyReplayFlag(rt, args, i, &opts)
			if code != 0 {
				return replayOptions{}, code
			}
			i = nextIndex
			continue
		}
		if code := setReplayTarget(rt, &opts, args[i]); code != 0 {
			return replayOptions{}, code
		}
	}
	if opts.target == "" {
		fmt.Fprintln(rt.Stderr, "szr: replay requires a tee id or file path")
		return replayOptions{}, 2
	}
	return opts, 0
}

func applyReplayFlag(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	switch args[index] {
	case "--json":
		opts.asJSON = true
		return index, 0
	case "--command":
		return setReplayStringOption(rt, args, index, "--command", &opts.commandText)
	case "--profile":
		return setReplayStringOption(rt, args, index, "--profile", &opts.profileName)
	case "--cwd":
		return setReplayStringOption(rt, args, index, "--cwd", &opts.overrideCwd)
	case "--exit-code":
		return setReplayExitCode(rt, args, index, opts)
	case "--max-lines":
		return setReplayMaxLines(rt, args, index, opts)
	default:
		fmt.Fprintf(rt.Stderr, "szr: unknown replay flag %s\n", args[index])
		return index, 2
	}
}

func setReplayStringOption(rt Runtime, args []string, index int, flag string, target *string) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, flag)
	if !ok {
		return index, 2
	}
	*target = value
	return index, 0
}

func setReplayExitCode(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, "--exit-code")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: invalid replay exit code %q\n", value)
		return index, 2
	}
	opts.overrideExitCode = parsed
	opts.overrideExitSet = true
	return index, 0
}

func setReplayMaxLines(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, "--max-lines")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		fmt.Fprintf(rt.Stderr, "szr: invalid replay max lines %q\n", value)
		return index, 2
	}
	opts.maxLines = parsed
	return index, 0
}

func setReplayTarget(rt Runtime, opts *replayOptions, target string) int {
	if opts.target != "" {
		fmt.Fprintln(rt.Stderr, "szr: replay accepts exactly one tee id or file path")
		return 2
	}
	opts.target = target
	return 0
}

func requireReplayValue(rt Runtime, args []string, index *int, flag string) (string, bool) {
	if *index+1 >= len(args) {
		fmt.Fprintf(rt.Stderr, "szr: replay requires a value after %s\n", flag)
		return "", false
	}
	*index = *index + 1
	return args[*index], true
}

func resolveReplayContext(opts replayOptions, entry teeindex.Entry, foundEntry bool) (string, string, int) {
	commandText := opts.commandText
	if commandText == "" && foundEntry {
		commandText = entry.Command
	}
	cwd, _ := os.Getwd()
	if opts.overrideCwd != "" {
		cwd = opts.overrideCwd
	} else if foundEntry && strings.TrimSpace(entry.Cwd) != "" {
		cwd = entry.Cwd
	}
	exitCode := 1
	if foundEntry {
		exitCode = entry.ExitCode
	}
	if opts.overrideExitSet {
		exitCode = opts.overrideExitCode
	}
	return commandText, cwd, exitCode
}

func captureExecution(ctx context.Context, command []string, cwd string) (engine.Execution, time.Duration, error) {
	if len(command) == 0 {
		return engine.Execution{}, 0, fmt.Errorf("missing command")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return engine.Execution{}, 0, err
		}
	}
	return engine.Execution{
		Command:  append([]string(nil), command...),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, duration, nil
}

func selectedProfile(eng *engine.Engine, inv engine.Invocation, override string) (engine.Profile, error) {
	if strings.TrimSpace(override) == "" {
		return eng.Explain(inv), nil
	}
	for _, profile := range eng.Profiles() {
		if profile.Name == override {
			return profile, nil
		}
	}
	return engine.Profile{}, fmt.Errorf("unknown profile %q", override)
}

func preparedCommand(profile engine.Profile, inv engine.Invocation) []string {
	if profile.Prepare != nil {
		return profile.Prepare(inv)
	}
	return append([]string(nil), inv.Command...)
}

func readReplayTarget(rt Runtime, target string) (string, teeindex.Entry, bool, error) {
	if data, err := os.ReadFile(target); err == nil {
		return string(data), teeindex.Entry{}, false, nil
	}
	store := teeindex.New(rt.Paths.TeeDir)
	entry, ok, err := store.Find(target)
	if err != nil {
		return "", teeindex.Entry{}, false, fmt.Errorf("failed to read tee index: %w", err)
	}
	if !ok {
		return "", teeindex.Entry{}, false, fmt.Errorf("replay target %q is neither a file nor a known tee artifact", target)
	}
	data, err := store.Read(entry)
	if err != nil {
		return "", teeindex.Entry{}, false, fmt.Errorf("tee artifact unavailable: %w", err)
	}
	return string(data), entry, true, nil
}

func executionForReplay(profile engine.Profile, raw string, exitCode int) engine.Execution {
	switch profile.StreamPreference {
	case engine.StreamStderrOnly, engine.StreamStderrFirst:
		return engine.Execution{Stderr: raw, ExitCode: exitCode}
	default:
		return engine.Execution{Stdout: raw, ExitCode: exitCode}
	}
}

func buildReplayOutput(command string, effectiveCommand []string, profile engine.Profile, maxLines int, exitCode int, rendered engine.RenderedExecution) replayOutput {
	return replayOutput{
		Command:           command,
		EffectiveCommand:  strings.Join(effectiveCommand, " "),
		Profile:           profile.Name,
		ProfileConfidence: profile.Confidence,
		ExitCode:          exitCode,
		FallbackUsed:      rendered.FallbackUsed,
		RawTokens:         rendered.RawTokens,
		FilteredTokens:    rendered.FilteredTokens,
		SavedTokens:       rendered.RawTokens - rendered.FilteredTokens,
		SavingsPct:        savingsPct(rendered.RawTokens, rendered.FilteredTokens),
		BytesParsed:       rendered.BytesParsed,
		BytesEmitted:      rendered.BytesEmitted,
		Budget:            engine.ResolveBudget(profile, engine.Invocation{Command: effectiveCommand, Display: effectiveCommand}, maxLines),
		Display:           rendered.Text,
	}
}

func savingsPct(rawTokens, filteredTokens int) float64 {
	if rawTokens <= 0 {
		return 0
	}
	return float64(rawTokens-filteredTokens) * 100 / float64(rawTokens)
}

func percentileInt64(values []int64, pct int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int((float64(pct) / 100) * float64(len(sorted)-1))
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func percent(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}
