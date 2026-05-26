package workflows

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/teeindex"
)

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
		Budget:            engine.ResolveBudget(profile, engine.Invocation{Command: effectiveCommand, Display: effectiveCommand, Advanced: config.Default().Advanced}, maxLines),
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
