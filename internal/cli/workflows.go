package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/filters"
	"szr/internal/history"
	"szr/internal/profiles"
	"szr/internal/rules"
	"szr/internal/teeindex"
)

type hotspotStat struct {
	Fingerprint   string  `json:"fingerprint"`
	Command       string  `json:"command"`
	Profile       string  `json:"profile"`
	Samples       int     `json:"samples"`
	AveragePct    float64 `json:"average_pct"`
	Failures      int     `json:"failures"`
	FailureRate   float64 `json:"failure_rate"`
	Fallbacks     int     `json:"fallbacks"`
	FallbackRate  float64 `json:"fallback_rate"`
	TeeCount      int     `json:"tee_count"`
	TeeRate       float64 `json:"tee_rate"`
	DurationP50MS int64   `json:"duration_p50_ms"`
	DurationP95MS int64   `json:"duration_p95_ms"`
}

type recommendation struct {
	Kind        string               `json:"kind"`
	Priority    int                  `json:"priority"`
	Command     string               `json:"command"`
	Profile     string               `json:"profile,omitempty"`
	Samples     int                  `json:"samples"`
	Confidence  string               `json:"confidence,omitempty"`
	Reason      string               `json:"reason"`
	Action      string               `json:"action"`
	Fingerprint string               `json:"fingerprint,omitempty"`
	Direction   string               `json:"direction,omitempty"`
	Suggested   history.BudgetTarget `json:"suggested,omitempty"`
}

type replayOutput struct {
	Command           string              `json:"command,omitempty"`
	EffectiveCommand  string              `json:"effective_command,omitempty"`
	Profile           string              `json:"profile"`
	ProfileConfidence string              `json:"profile_confidence,omitempty"`
	ExitCode          int                 `json:"exit_code"`
	FallbackUsed      bool                `json:"fallback_used"`
	RawTokens         int                 `json:"raw_tokens"`
	FilteredTokens    int                 `json:"filtered_tokens"`
	SavedTokens       int                 `json:"saved_tokens"`
	SavingsPct        float64             `json:"savings_pct"`
	BytesParsed       int                 `json:"bytes_parsed"`
	BytesEmitted      int                 `json:"bytes_emitted"`
	Budget            engine.OutputBudget `json:"budget"`
	Display           string              `json:"display"`
}

type compareOutput struct {
	Command           string              `json:"command"`
	EffectiveCommand  string              `json:"effective_command"`
	Profile           string              `json:"profile"`
	ProfileConfidence string              `json:"profile_confidence,omitempty"`
	ExitCode          int                 `json:"exit_code"`
	DurationMS        int64               `json:"duration_ms"`
	FallbackUsed      bool                `json:"fallback_used"`
	RawTokens         int                 `json:"raw_tokens"`
	FilteredTokens    int                 `json:"filtered_tokens"`
	SavedTokens       int                 `json:"saved_tokens"`
	SavingsPct        float64             `json:"savings_pct"`
	BytesParsed       int                 `json:"bytes_parsed"`
	BytesEmitted      int                 `json:"bytes_emitted"`
	Budget            engine.OutputBudget `json:"budget"`
	RawPreview        string              `json:"raw_preview"`
	ReducedPreview    string              `json:"reduced_preview"`
}

func (a *App) runRecommend(args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: recommend requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid recommend limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown recommend flag %s\n", args[i])
			return 2
		}
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	recommendations := buildRecommendations(records, limit)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(recommendations)
		return 0
	}
	if len(recommendations) == 0 {
		fmt.Println("no recommendations yet")
		return 0
	}

	fmt.Println("recommendations:")
	for _, item := range recommendations {
		fmt.Printf("  - [%s] %s\n", item.Kind, item.Command)
		fmt.Printf("    reason: %s\n", item.Reason)
		fmt.Printf("    action: %s\n", item.Action)
		if item.Profile != "" || item.Samples > 0 || item.Confidence != "" {
			fmt.Printf("    profile=%s samples=%d confidence=%s\n", item.Profile, item.Samples, emptyDash(item.Confidence))
		}
		if item.Direction != "" {
			fmt.Printf(
				"    target: %s lines=%d bytes=%d tokens=%d\n",
				item.Direction,
				item.Suggested.MaxLines,
				item.Suggested.MaxBytes,
				item.Suggested.MaxTokens,
			)
		}
	}
	return 0
}

func (a *App) runHotspots(args []string) int {
	asJSON := false
	limit := 8
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: hotspots requires a value after --limit")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid hotspots limit %q\n", args[i])
				return 2
			}
			limit = value
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown hotspots flag %s\n", args[i])
			return 2
		}
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	hotspots := buildHotspots(records, limit)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(hotspots)
		return 0
	}
	if len(hotspots) == 0 {
		fmt.Println("no hotspots yet")
		return 0
	}

	fmt.Println("hotspots:")
	for _, item := range hotspots {
		fmt.Printf(
			"  - %s  profile=%s samples=%d avg=%.1f%% fallback=%.1f%% fail=%.1f%% tee=%.1f%% p50/p95=%d/%dms\n",
			item.Command,
			item.Profile,
			item.Samples,
			item.AveragePct,
			item.FallbackRate,
			item.FailureRate,
			item.TeeRate,
			item.DurationP50MS,
			item.DurationP95MS,
		)
	}
	return 0
}

func (a *App) runReplay(flags globalFlags, args []string) int {
	opts, code := parseReplayArgs(args)
	if code != 0 {
		return code
	}
	raw, entry, foundEntry, err := a.readReplayTarget(opts.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}

	commandText, cwd, exitCode := resolveReplayContext(opts, entry, foundEntry)
	if commandText == "" && opts.profileName == "" {
		fmt.Fprintln(os.Stderr, "szr: replay requires --command or --profile when replaying a plain file")
		return 2
	}

	cfg := a.configForFlags(flags)
	if opts.maxLines > 0 {
		cfg.MaxPreviewLines = opts.maxLines
	}
	eng := engine.New(cfg, a.paths, a.history, engineProfilesForConfig(cfg))

	inv := engine.Invocation{
		Command:             strings.Fields(commandText),
		Display:             strings.Fields(commandText),
		Cwd:                 cwd,
		Verbose:             flags.verbose,
		UltraCompact:        flags.ultra,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
	}
	effectiveInv, _ := eng.ExplainPreferences(inv)
	profile, err := selectedProfile(eng, inv, opts.profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 2
	}

	execResult := executionForReplay(profile, raw, exitCode)
	rendered := engine.RenderExecution(profile, effectiveInv, execResult, cfg.MaxPreviewLines, false)
	output := buildReplayOutput(commandText, effectiveInv.Command, profile, cfg.MaxPreviewLines, execResult.ExitCode, rendered)
	if opts.asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(output)
		return 0
	}

	if commandText != "" {
		fmt.Printf("command: %s\n", commandText)
	}
	if len(effectiveInv.Command) > 0 {
		fmt.Printf("effective command: %s\n", strings.Join(effectiveInv.Command, " "))
	}
	fmt.Printf("profile: %s\n", profile.Name)
	if profile.Confidence != "" {
		fmt.Printf("confidence: %s\n", profile.Confidence)
	}
	fmt.Printf("exit: %d\n", execResult.ExitCode)
	fmt.Printf("fallback: %t\n", output.FallbackUsed)
	fmt.Printf(
		"tokens: raw=%d out=%d saved=%d (%.1f%%)\n",
		output.RawTokens,
		output.FilteredTokens,
		output.SavedTokens,
		output.SavingsPct,
	)
	fmt.Printf("budget: lines=%d bytes=%d tokens=%d\n", output.Budget.MaxLines, output.Budget.MaxBytes, output.Budget.MaxTokens)
	fmt.Println("rendered:")
	fmt.Println(output.Display)
	return 0
}

type replayOptions struct {
	asJSON           bool
	commandText      string
	profileName      string
	overrideExitCode int
	overrideExitSet  bool
	overrideCwd      string
	maxLines         int
	target           string
}

func parseReplayArgs(args []string) (replayOptions, int) {
	opts := replayOptions{}
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			nextIndex, code := applyReplayFlag(args, i, &opts)
			if code != 0 {
				return replayOptions{}, code
			}
			i = nextIndex
			continue
		}
		if code := setReplayTarget(&opts, args[i]); code != 0 {
			return replayOptions{}, code
		}
	}
	if opts.target == "" {
		fmt.Fprintln(os.Stderr, "szr: replay requires a tee id or file path")
		return replayOptions{}, 2
	}
	return opts, 0
}

func applyReplayFlag(args []string, index int, opts *replayOptions) (int, int) {
	switch args[index] {
	case "--json":
		opts.asJSON = true
		return index, 0
	case "--command":
		return setReplayStringOption(args, index, "--command", &opts.commandText)
	case "--profile":
		return setReplayStringOption(args, index, "--profile", &opts.profileName)
	case "--cwd":
		return setReplayStringOption(args, index, "--cwd", &opts.overrideCwd)
	case "--exit-code":
		return setReplayExitCode(args, index, opts)
	case "--max-lines":
		return setReplayMaxLines(args, index, opts)
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown replay flag %s\n", args[index])
		return index, 2
	}
}

func setReplayStringOption(args []string, index int, flag string, target *string) (int, int) {
	value, ok := requireReplayValue(args, &index, flag)
	if !ok {
		return index, 2
	}
	*target = value
	return index, 0
}

func setReplayExitCode(args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(args, &index, "--exit-code")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: invalid replay exit code %q\n", value)
		return index, 2
	}
	opts.overrideExitCode = parsed
	opts.overrideExitSet = true
	return index, 0
}

func setReplayMaxLines(args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(args, &index, "--max-lines")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		fmt.Fprintf(os.Stderr, "szr: invalid replay max lines %q\n", value)
		return index, 2
	}
	opts.maxLines = parsed
	return index, 0
}

func setReplayTarget(opts *replayOptions, target string) int {
	if opts.target != "" {
		fmt.Fprintln(os.Stderr, "szr: replay accepts exactly one tee id or file path")
		return 2
	}
	opts.target = target
	return 0
}

func requireReplayValue(args []string, index *int, flag string) (string, bool) {
	if *index+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "szr: replay requires a value after %s\n", flag)
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

func (a *App) runCompare(ctx context.Context, flags globalFlags, args []string) int {
	asJSON := false
	maxLines := 0
	command := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--max-lines":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: compare requires a value after --max-lines")
				return 2
			}
			i++
			value, err := strconv.Atoi(args[i])
			if err != nil || value <= 0 {
				fmt.Fprintf(os.Stderr, "szr: invalid compare max lines %q\n", args[i])
				return 2
			}
			maxLines = value
		default:
			if len(command) == 0 && strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown compare flag %s\n", args[i])
				return 2
			}
			command = append(command, args[i])
		}
	}
	if len(command) == 0 {
		fmt.Fprintln(os.Stderr, "szr: compare requires a command")
		return 2
	}

	cfg := a.configForFlags(flags)
	if maxLines > 0 {
		cfg.MaxPreviewLines = maxLines
	}
	eng := engine.New(cfg, a.paths, a.history, engineProfilesForConfig(cfg))

	cwd, _ := os.Getwd()
	inv := engine.Invocation{
		Command:             append([]string(nil), command...),
		Display:             append([]string(nil), command...),
		Cwd:                 cwd,
		Verbose:             flags.verbose,
		UltraCompact:        flags.ultra,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
	}
	effectiveInv, _ := eng.ExplainPreferences(inv)
	profile := eng.Explain(inv)
	effectiveCommand := preparedCommand(profile, effectiveInv)

	execResult, duration, err := captureExecution(ctx, effectiveCommand, cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
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
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return 0
	}

	fmt.Printf("command: %s\n", out.Command)
	fmt.Printf("effective command: %s\n", out.EffectiveCommand)
	fmt.Printf("profile: %s\n", out.Profile)
	if out.ProfileConfidence != "" {
		fmt.Printf("confidence: %s\n", out.ProfileConfidence)
	}
	fmt.Printf("duration: %dms\n", out.DurationMS)
	fmt.Printf("exit: %d\n", out.ExitCode)
	fmt.Printf("fallback: %t\n", out.FallbackUsed)
	fmt.Printf(
		"tokens: raw=%d out=%d saved=%d (%.1f%%)\n",
		out.RawTokens,
		out.FilteredTokens,
		out.SavedTokens,
		out.SavingsPct,
	)
	fmt.Printf("budget: lines=%d bytes=%d tokens=%d\n", out.Budget.MaxLines, out.Budget.MaxBytes, out.Budget.MaxTokens)
	fmt.Println("raw preview:")
	fmt.Println(out.RawPreview)
	fmt.Println("reduced preview:")
	fmt.Println(out.ReducedPreview)
	return 0
}

func (a *App) runRules(flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: rules requires a subcommand")
		return 2
	}
	switch args[0] {
	case "check":
		return a.runRulesCheck(args[1:])
	case "test":
		return a.runRulesTest(flags, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown rules subcommand %s\n", args[0])
		return 2
	}
}

func (a *App) runRulesCheck(args []string) int {
	asJSON := false
	path := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			asJSON = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown rules check flag %s\n", arg)
				return 2
			}
			if path != "" {
				fmt.Fprintln(os.Stderr, "szr: rules check accepts at most one path")
				return 2
			}
			path = arg
		}
	}

	resolved, file, err := loadRulesFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	if asJSON {
		payload := map[string]any{
			"path":        resolved,
			"profiles":    len(file.Profiles),
			"preferences": len(file.Preferences),
			"version":     file.Version,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

	fmt.Printf("rules: %s\n", resolved)
	fmt.Printf("version: %d\n", file.Version)
	fmt.Printf("profiles: %d\n", len(file.Profiles))
	fmt.Printf("preferences: %d\n", len(file.Preferences))
	fmt.Println("status: valid")
	return 0
}

func (a *App) runRulesTest(flags globalFlags, args []string) int {
	asJSON := false
	path := ""
	command := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			asJSON = true
		case "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: rules test requires a value after --file")
				return 2
			}
			i++
			path = args[i]
		default:
			command = append(command, args[i])
		}
	}
	if len(command) == 0 {
		fmt.Fprintln(os.Stderr, "szr: rules test requires a command")
		return 2
	}

	resolved, file, err := loadRulesFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	cfg := a.configForFlags(flags)
	cfg.ProjectRules = file
	paths := a.paths
	paths.ProjectRuleFile = resolved
	paths.ProjectDir = filepath.Dir(resolved)
	eng := engine.New(cfg, paths, a.history, engineProfilesForConfig(cfg))

	cwd, _ := os.Getwd()
	inv := engine.Invocation{
		Command:             append([]string(nil), command...),
		Display:             append([]string(nil), command...),
		Cwd:                 cwd,
		Verbose:             flags.verbose,
		UltraCompact:        flags.ultra,
		ReasoningBudgetMode: cfg.ReasoningBudgetMode,
	}
	effectiveInv, preferences := eng.ExplainPreferences(inv)
	profile := eng.Explain(inv)
	decisions := eng.ExplainDecisions(inv)

	if asJSON {
		payload := map[string]any{
			"rules":             resolved,
			"command":           strings.Join(command, " "),
			"effective_command": strings.Join(effectiveInv.Command, " "),
			"profile":           profile.Name,
			"source":            describeProfileSource(profile.Source, resolved),
			"preferences":       preferences,
			"decisions":         decisions,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

	fmt.Printf("rules: %s\n", resolved)
	fmt.Printf("command: %s\n", strings.Join(command, " "))
	if len(effectiveInv.Command) > 0 {
		fmt.Printf("effective command: %s\n", strings.Join(effectiveInv.Command, " "))
	}
	fmt.Printf("selected profile: %s\n", profile.Name)
	fmt.Printf("source: %s\n", describeProfileSource(profile.Source, resolved))
	if len(preferences) > 0 {
		fmt.Println("preferences:")
		for _, preference := range preferences {
			label := "satisfied"
			if preference.Applied {
				label = "applied"
			}
			fmt.Printf("  - %s %s\n", label, preference.Name)
		}
	}
	if len(decisions) > 0 {
		fmt.Println("matches:")
		for _, decision := range decisions {
			label := "also matches"
			if decision.Selected {
				label = "selected"
			}
			fmt.Printf("  - %s %s (%s)\n", label, decision.Name, describeProfileSource(decision.Source, resolved))
		}
	}
	return 0
}

func (a *App) runScaffold(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: scaffold requires a subcommand")
		return 2
	}
	switch args[0] {
	case "profile":
		return a.runScaffoldProfile(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown scaffold subcommand %s\n", args[0])
		return 2
	}
}

func (a *App) runScaffoldProfile(args []string) int {
	printOnly := false
	builtin := false
	name := ""
	for _, arg := range args {
		switch arg {
		case "--print":
			printOnly = true
		case "--builtin":
			builtin = true
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "szr: unknown scaffold profile flag %s\n", arg)
				return 2
			}
			if name != "" {
				fmt.Fprintln(os.Stderr, "szr: scaffold profile accepts exactly one profile name")
				return 2
			}
			name = arg
		}
	}
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr, "szr: scaffold profile requires a name")
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	files := scaffoldProfileFiles(cwd, name, builtin)
	if printOnly {
		fmt.Printf("plan: scaffold profile %s\n", name)
		for path, content := range files {
			fmt.Printf("  %s (%d bytes)\n", relativeToRepo(cwd, path), len(content))
		}
		return 0
	}
	for path, content := range files {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "szr: scaffold target already exists: %s\n", path)
			return 1
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to create %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to write %s: %v\n", path, err)
			return 1
		}
	}
	fmt.Printf("scaffolded profile %s\n", name)
	for path := range files {
		fmt.Printf("  %s\n", relativeToRepo(cwd, path))
	}
	return 0
}

func buildRecommendations(records []history.Record, limit int) []recommendation {
	if len(records) == 0 {
		return nil
	}
	hotspots := buildHotspots(records, limit*2)
	suggestions := history.SuggestBudgets(records, history.BudgetSuggestionOptions{Limit: limit * 2})
	items := make([]recommendation, 0, len(suggestions)+len(hotspots))
	seen := map[string]struct{}{}

	for _, suggestion := range suggestions {
		item := recommendationForBudget(suggestion)
		items = append(items, item)
		seen[recommendationKey(item)] = struct{}{}
	}

	for _, hotspot := range hotspots {
		appendHotspotRecommendations(&items, seen, hotspot)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			if items[i].Samples == items[j].Samples {
				return items[i].Command < items[j].Command
			}
			return items[i].Samples > items[j].Samples
		}
		return items[i].Priority > items[j].Priority
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func recommendationForBudget(suggestion history.BudgetSuggestion) recommendation {
	return recommendation{
		Kind:        "budget",
		Priority:    recommendationPriorityForBudget(suggestion),
		Command:     suggestion.Command,
		Profile:     suggestion.Profile,
		Samples:     suggestion.Samples,
		Confidence:  suggestion.Confidence,
		Reason:      strings.ReplaceAll(string(suggestion.Reason), "_", " "),
		Action:      fmt.Sprintf("adjust the active budget to lines=%d bytes=%d tokens=%d", suggestion.Suggested.MaxLines, suggestion.Suggested.MaxBytes, suggestion.Suggested.MaxTokens),
		Fingerprint: suggestion.Fingerprint,
		Direction:   string(suggestion.Direction),
		Suggested:   suggestion.Suggested,
	}
}

func appendHotspotRecommendations(items *[]recommendation, seen map[string]struct{}, hotspot hotspotStat) {
	if hotspot.Samples < 2 {
		return
	}
	for _, item := range hotspotRecommendations(hotspot) {
		key := recommendationKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		*items = append(*items, item)
		seen[key] = struct{}{}
	}
}

func hotspotRecommendations(hotspot hotspotStat) []recommendation {
	items := []recommendation{}
	if customProfileRecommendation(hotspot).Kind != "" {
		items = append(items, customProfileRecommendation(hotspot))
	}
	if item, ok := structuredRewriteRecommendation(hotspot); ok {
		items = append(items, item)
	}
	if item, ok := teeReviewRecommendation(hotspot); ok {
		items = append(items, item)
	}
	return items
}

func customProfileRecommendation(hotspot hotspotStat) recommendation {
	if !isGenericHotspot(hotspot) || hotspot.FallbackRate < 0 {
		return recommendation{}
	}
	return recommendation{
		Kind:        "custom-profile",
		Priority:    70,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      fmt.Sprintf("%s still routes through %s after %d runs", hotspot.Command, hotspot.Profile, hotspot.Samples),
		Action:      "add a project-local profile or builtin reducer so this command stops relying on the generic path",
		Fingerprint: hotspot.Fingerprint,
	}
}

func structuredRewriteRecommendation(hotspot hotspotStat) (recommendation, bool) {
	if !isGenericHotspot(hotspot) {
		return recommendation{}, false
	}
	hint := structuredHint(hotspot.Command)
	if hint == "" {
		return recommendation{}, false
	}
	return recommendation{
		Kind:        "structured-rewrite",
		Priority:    65,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "medium",
		Reason:      "this command family usually benefits from a deterministic machine-readable mode",
		Action:      hint,
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func teeReviewRecommendation(hotspot hotspotStat) (recommendation, bool) {
	if hotspot.Failures <= 0 || hotspot.TeeRate < 50 {
		return recommendation{}, false
	}
	return recommendation{
		Kind:        "tee-review",
		Priority:    50,
		Command:     hotspot.Command,
		Profile:     hotspot.Profile,
		Samples:     hotspot.Samples,
		Confidence:  "low",
		Reason:      "failing runs are frequently preserving full artifacts",
		Action:      fmt.Sprintf("inspect preserved failures with `szr tee find %q` before adding another reducer", firstWordOrCommand(hotspot.Command)),
		Fingerprint: hotspot.Fingerprint,
	}, true
}

func isGenericHotspot(hotspot hotspotStat) bool {
	return hotspot.Profile == "passthrough" || strings.HasPrefix(hotspot.Profile, "generic-")
}

func recommendationKey(item recommendation) string {
	return item.Kind + ":" + item.Fingerprint
}

func buildHotspots(records []history.Record, limit int) []hotspotStat {
	if len(records) == 0 {
		return nil
	}
	type aggregate struct {
		stat      hotspotStat
		durations []int64
	}
	grouped := map[string]*aggregate{}
	for _, raw := range records {
		rec := raw
		if strings.TrimSpace(rec.CommandFingerprint) == "" {
			rec.CommandFingerprint = history.Fingerprint(rec.Command)
		}
		if rec.CommandFingerprint == "" {
			continue
		}
		item := grouped[rec.CommandFingerprint]
		if item == nil {
			item = &aggregate{stat: hotspotStat{
				Fingerprint: rec.CommandFingerprint,
				Command:     rec.Command,
				Profile:     rec.Profile,
			}}
			grouped[rec.CommandFingerprint] = item
		}
		item.stat.Samples++
		item.stat.AveragePct += rec.SavingsPct
		if rec.ExitCode != 0 {
			item.stat.Failures++
		}
		if rec.FallbackUsed {
			item.stat.Fallbacks++
		}
		if rec.TeePath != "" {
			item.stat.TeeCount++
		}
		item.durations = append(item.durations, rec.DurationMS)
	}

	list := make([]hotspotStat, 0, len(grouped))
	for _, item := range grouped {
		item.stat.AveragePct /= float64(item.stat.Samples)
		item.stat.FailureRate = percent(item.stat.Failures, item.stat.Samples)
		item.stat.FallbackRate = percent(item.stat.Fallbacks, item.stat.Samples)
		item.stat.TeeRate = percent(item.stat.TeeCount, item.stat.Samples)
		item.stat.DurationP50MS = percentileInt64(item.durations, 50)
		item.stat.DurationP95MS = percentileInt64(item.durations, 95)
		list = append(list, item.stat)
	}
	sort.Slice(list, func(i, j int) bool {
		leftScore := hotspotSeverity(list[i])
		rightScore := hotspotSeverity(list[j])
		if leftScore == rightScore {
			if list[i].Samples == list[j].Samples {
				return list[i].Command < list[j].Command
			}
			return list[i].Samples > list[j].Samples
		}
		return leftScore > rightScore
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}

func hotspotSeverity(item hotspotStat) float64 {
	return (100 - item.AveragePct) + item.FallbackRate + item.FailureRate/2 + item.TeeRate/4 + float64(item.DurationP95MS)/25
}

func recommendationPriorityForBudget(item history.BudgetSuggestion) int {
	switch item.Reason {
	case history.BudgetSuggestionFallbackHeavy:
		return 90
	case history.BudgetSuggestionAggressiveCompression:
		return 80
	default:
		return 75
	}
}

func structuredHint(command string) string {
	normalized := strings.Fields(strings.ToLower(command))
	if len(normalized) == 0 {
		return ""
	}
	if normalized[0] == "szr" && len(normalized) > 1 {
		normalized = normalized[1:]
	}
	if len(normalized) == 0 {
		return ""
	}
	switch normalized[0] {
	case "terraform", "tofu":
		return "prefer JSON-capable flows like `plan -json`, `show -json`, or structured state output via a project preference"
	case "gh":
		return "prefer `--json` with explicit fields or list/view subcommands that emit stable machine-readable output"
	case "eslint":
		return "prefer `-f json` so szr can reduce file- and rule-level diagnostics deterministically"
	case "tsc":
		return "prefer `--pretty false` and narrower file targets so output is stable and reducer-friendly"
	case "kubectl":
		return "prefer `-o json` or other structured output where the subcommand supports it"
	}
	if len(normalized) >= 2 && normalized[0] == "docker" && normalized[1] == "build" {
		return "prefer deterministic plain progress or explicit metadata flags before relying on the generic reducer"
	}
	return ""
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

func (a *App) readReplayTarget(target string) (string, teeindex.Entry, bool, error) {
	if data, err := os.ReadFile(target); err == nil {
		return string(data), teeindex.Entry{}, false, nil
	}
	store := teeindex.New(a.paths.TeeDir)
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

func loadRulesFile(path string) (string, rules.File, error) {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", rules.File{}, err
		}
		discovered, _, err := rules.Discover(cwd)
		if err != nil {
			return "", rules.File{}, err
		}
		if discovered == "" {
			return "", rules.File{}, fmt.Errorf("no .szr.json/.szr.yaml/.szr.yml file found from %s upward", cwd)
		}
		resolved = discovered
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return "", rules.File{}, err
	}
	file, err := rules.ParseFile(resolved, data)
	if err != nil {
		return "", rules.File{}, err
	}
	return resolved, file, nil
}

func scaffoldProfileFiles(root, name string, builtin bool) map[string]string {
	slug := sanitizeSlug(name)
	files := map[string]string{}
	if builtin {
		files[filepath.Join(root, "internal", "profiles", slug, "profile.go")] = builtinProfileStub(slug)
		files[filepath.Join(root, "test", "profiles", slug, "render_test.go")] = builtinProfileTestStub(slug)
		return files
	}
	base := filepath.Join(root, ".szr", "scaffold", slug)
	files[filepath.Join(base, "profile.yaml")] = customProfileStub(slug)
	files[filepath.Join(base, "raw.stdout.txt")] = "replace with representative raw stdout for this command family\n"
	files[filepath.Join(base, "raw.stderr.txt")] = "replace with representative raw stderr for this command family\n"
	files[filepath.Join(base, "expected.txt")] = "replace with the reducer output you want to preserve\n"
	files[filepath.Join(base, "README.md")] = scaffoldReadme(slug)
	return files
}

func customProfileStub(name string) string {
	return fmt.Sprintf(`version: 1
profiles:
  - name: %s
    description: Summarize the %s command family for agent-friendly review.
    explain:
      - Matches the target command family.
      - Rewrites the command into a more structured form before rendering compact output.
    match:
      command_prefix:
        - your-cli
        - subcommand
    rewrite:
      mode: append
      args:
        - --json
    render:
      mode: failure
      max_lines: 12
`, name, name)
}

func builtinProfileStub(name string) string {
	return fmt.Sprintf(`package %s

import "szr/internal/engine"

func Profiles(maxLines int) []engine.Profile {
	return []engine.Profile{{
		Name:        %q,
		Description: "Summarizes %s output into an agent-friendly preview.",
		Confidence:  engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Command) >= 2 && inv.Command[0] == "your-cli" && inv.Command[1] == "subcommand"
		},
		Prepare: func(inv engine.Invocation) []string {
			return append([]string(nil), inv.Command...)
		},
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return exec.Stdout
		},
		Explain: []string{
			"Matches the intended command family explicitly.",
			"Preserves the minimum set of identifiers and failure details needed for follow-up actions.",
		},
	}}
}
`, name, name, name)
}

func builtinProfileTestStub(name string) string {
	return fmt.Sprintf(`package %s_test

import (
	"testing"

	"szr/test/testutil"
)

func Test%sStub(t *testing.T) {
	_ = testutil.NewTestApp(t)
	t.Skip("replace scaffolded stub with reducer coverage")
}
`, name, toTitle(name))
}

func scaffoldReadme(name string) string {
	return fmt.Sprintf(`# %s scaffold

1. Capture representative command output into `+"`raw.stdout.txt`"+` and `+"`raw.stderr.txt`"+`.
2. Update `+"`profile.yaml`"+` or move the logic into a builtin profile.
3. Replace `+"`expected.txt`"+` with the reduced output contract you want tests to enforce.
`, name)
}

func sanitizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	if value == "" {
		return "profile"
	}
	return value
}

func toTitle(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func engineProfilesForConfig(cfg config.Config) []engine.Profile {
	return profiles.Builtins(cfg.MaxPreviewLines)
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

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func firstWordOrCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return command
	}
	return fields[0]
}
