package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		profiles: profiles,
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

func (e *Engine) match(inv Invocation) Profile {
	for _, profile := range e.profiles {
		if profile.Match != nil && profile.Match(inv) {
			return profile
		}
	}
	return Profile{
		Name:        "passthrough",
		Description: "Raw command passthrough with trimming.",
		Render: func(_ Invocation, exec Execution) string {
			return combineStreams(exec.Stdout, exec.Stderr)
		},
		Explain: []string{
			"No specialized profile matched.",
			"Raw stdout and stderr are combined with minimal trimming.",
		},
	}
}

func (e *Engine) writeTee(raw string, command []string) (string, error) {
	name := fmt.Sprintf("%d_%s.log", time.Now().Unix(), sanitizeFileName(strings.Join(command, "_")))
	path := filepath.Join(e.paths.TeeDir, name)
	return path, os.WriteFile(path, []byte(raw), 0o644)
}

func runCommand(ctx context.Context, args []string, cwd string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, nil
	}

	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		return stdout.String(), stderr.String(), exitCode, nil
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func combineStreams(stdout, stderr string) string {
	return CombineStreams(stdout, stderr)
}

func CombineStreams(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

func sanitizeFileName(value string) string {
	return SanitizeFileName(value)
}

func SanitizeFileName(value string) string {
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, value)
	value = strings.Trim(value, "_")
	if value == "" {
		return "output"
	}
	if len(value) > 48 {
		return value[:48]
	}
	return value
}
