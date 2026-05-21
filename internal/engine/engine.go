package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"szr/internal/config"
	"szr/internal/history"
)

type Engine struct {
	config   config.Config
	paths    config.Paths
	history  *history.Store
	profiles []Profile
}

func New(cfg config.Config, paths config.Paths, store *history.Store, profiles []Profile) *Engine {
	return &Engine{
		config:   cfg,
		paths:    paths,
		history:  store,
		profiles: mergeProfiles(compileRuleProfiles(cfg), profiles),
	}
}

func (e *Engine) Profiles() []Profile {
	return append([]Profile(nil), e.profiles...)
}

func (e *Engine) Explain(inv Invocation) Profile {
	return e.match(inv)
}

func (e *Engine) Execute(ctx context.Context, inv Invocation, passthrough bool) (Result, error) {
	if len(inv.Command) == 0 {
		return Result{}, fmt.Errorf("missing command")
	}

	profile := e.match(inv)
	command := inv.Command
	if profile.Prepare != nil {
		command = profile.Prepare(inv)
	}

	start := time.Now()
	stdout, stderr, exitCode, err := runCommand(ctx, command, inv.Cwd)
	duration := time.Since(start)
	rawCombined := combineStreams(stdout, stderr)
	rendered := rawCombined
	if !passthrough && profile.Render != nil {
		rendered = profile.Render(inv, Execution{
			Command:  command,
			Stdout:   stdout,
			Stderr:   stderr,
			ExitCode: exitCode,
			Duration: duration,
		})
	}
	if strings.TrimSpace(rendered) == "" {
		rendered = rawCombined
	}

	teePath := ""
	if exitCode != 0 && e.config.TeeOnFailure && rawCombined != "" {
		path, teeErr := e.writeTee(rawCombined, command)
		if teeErr == nil {
			teePath = path
			if !passthrough {
				rendered = strings.TrimRight(rendered, "\n") + "\n[full output: " + teePath + "]"
			}
		}
	}

	record := history.Record{
		Timestamp:      time.Now(),
		Command:        strings.Join(inv.Display, " "),
		Profile:        profile.Name,
		Cwd:            inv.Cwd,
		DurationMS:     duration.Milliseconds(),
		ExitCode:       exitCode,
		RawBytes:       len(rawCombined),
		FilteredBytes:  len(rendered),
		RawTokens:      history.EstimateTokens(rawCombined),
		FilteredTokens: history.EstimateTokens(rendered),
		TeePath:        teePath,
	}
	record.SavedTokens = record.RawTokens - record.FilteredTokens
	if record.RawTokens > 0 {
		record.SavingsPct = float64(record.SavedTokens) * 100 / float64(record.RawTokens)
	}
	_ = e.history.Append(record)

	result := Result{
		ProfileName: profile.Name,
		Display:     strings.TrimRight(rendered, "\n"),
		RawCombined: rawCombined,
		ExitCode:    exitCode,
		TeePath:     teePath,
		Duration:    duration,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}
