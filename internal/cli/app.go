package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/filters"
	"szr/internal/history"
	"szr/internal/profiles"
)

type App struct {
	version string
	config  config.Config
	paths   config.Paths
	history *history.Store
	engine  *engine.Engine
}

type globalFlags struct {
	verbose int
	ultra   bool
}

func New(version string) *App {
	cfg, paths, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to load config: %v\n", err)
		os.Exit(1)
	}
	store := history.New(paths.HistoryFile)
	return &App{
		version: version,
		config:  cfg,
		paths:   paths,
		history: store,
		engine:  engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines)),
	}
}

func (a *App) Run(ctx context.Context, args []string) int {
	flags, rest, err := parseGlobalFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(rest) == 0 {
		a.printHelp()
		return 0
	}

	switch rest[0] {
	case "--help", "-h", "help":
		a.printHelp()
		return 0
	case "--version", "version":
		fmt.Println("szr", a.version)
		return 0
	case "gain":
		return a.runGain(rest[1:])
	case "profiles":
		return a.runProfiles()
	case "doctor":
		return a.runDoctor()
	case "proxy":
		return a.runExternal(ctx, flags, "proxy", rest[1:], true)
	case "run":
		return a.runExternal(ctx, flags, "run", rest[1:], false)
	case "test":
		return a.runExternal(ctx, flags, "test", rest[1:], false)
	case "summary":
		return a.runExternal(ctx, flags, "summary", rest[1:], false)
	case "explain":
		return a.runExplain(flags, rest[1:])
	case "git", "go":
		return a.runExternal(ctx, flags, rest[0], rest[1:], false)
	case "ls":
		return a.runLS(rest[1:])
	case "read":
		return a.runRead(rest[1:])
	case "grep":
		return a.runGrep(rest[1:])
	case "json":
		return a.runJSON(rest[1:])
	case "log":
		return a.runLog(rest[1:])
	default:
		return a.runExternal(ctx, flags, "run", rest, false)
	}
}

func parseGlobalFlags(args []string) (globalFlags, []string, error) {
	var flags globalFlags
	for len(args) > 0 {
		switch args[0] {
		case "-u", "--ultra-compact":
			flags.ultra = true
			args = args[1:]
		case "-v", "--verbose":
			flags.verbose++
			args = args[1:]
		case "-vv":
			flags.verbose += 2
			args = args[1:]
		case "-vvv":
			flags.verbose += 3
			args = args[1:]
		default:
			if strings.HasPrefix(args[0], "-") && strings.Trim(args[0], "v") == "-" {
				flags.verbose += strings.Count(args[0], "v")
				args = args[1:]
				continue
			}
			return flags, args, nil
		}
	}
	return flags, args, nil
}

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
		fmt.Fprintf(os.Stderr, "[szr] profile=%s duration=%s exit=%d\n", result.ProfileName, result.Duration.Round(time.Millisecond), result.ExitCode)
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
	display := args
	command := args
	if args[0] == "git" || args[0] == "go" {
		command = args
	} else {
		command = args
	}
	cwd, _ := os.Getwd()
	profile := a.engine.Explain(engine.Invocation{
		Command:      command,
		Display:      display,
		Cwd:          cwd,
		Verbose:      flags.verbose,
		UltraCompact: flags.ultra,
	})
	fmt.Printf("profile: %s\n", profile.Name)
	fmt.Printf("about: %s\n", profile.Description)
	for _, line := range profile.Explain {
		fmt.Printf("- %s\n", line)
	}
	return 0
}

func (a *App) runGain(args []string) int {
	showHistory := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown gain flag %s\n", arg)
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
	fmt.Printf("failures: %d\n", summary.Failures)
	if len(summary.TopCommands) > 0 {
		fmt.Println("top commands:")
		for _, cmd := range summary.TopCommands {
			fmt.Printf("  %s (%d)\n", cmd.Command, cmd.Count)
		}
	}
	if showHistory {
		fmt.Println("recent:")
		for _, rec := range summary.Recent {
			fmt.Printf("  %s  %s  %s  %.1f%%\n", rec.Timestamp.Format(time.RFC3339), rec.Profile, rec.Command, rec.SavingsPct)
		}
	}
	return 0
}

func (a *App) runProfiles() int {
	for _, profile := range a.engine.Profiles() {
		fmt.Printf("%s\n  %s\n", profile.Name, profile.Description)
	}
	return 0
}

func (a *App) runDoctor() int {
	fmt.Printf("version: %s\n", a.version)
	fmt.Printf("config: %s\n", a.paths.ConfigFile)
	fmt.Printf("history: %s\n", a.paths.HistoryFile)
	fmt.Printf("tee dir: %s\n", a.paths.TeeDir)
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

func (a *App) runLS(args []string) int {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		paths = append(paths, path)
		if info.IsDir() && len(strings.Split(filepath.Clean(path), string(filepath.Separator)))-len(strings.Split(filepath.Clean(root), string(filepath.Separator))) > 2 {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Println(filters.BuildTree(paths, root))
	return 0
}

func (a *App) runRead(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: read requires a file")
		return 2
	}
	level := "none"
	lineNumbers := false
	maxLines := a.config.MaxPreviewLines
	files := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-l", "--level":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: missing value for --level")
				return 2
			}
			level = args[i]
		case "-n", "--line-numbers":
			lineNumbers = true
		case "--max-lines":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: missing value for --max-lines")
				return 2
			}
			fmt.Sscanf(args[i], "%d", &maxLines)
		default:
			files = append(files, args[i])
		}
	}
	for idx, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return 1
		}
		if idx > 0 {
			fmt.Println()
		}
		if len(files) > 1 {
			fmt.Printf("== %s ==\n", file)
		}
		fmt.Println(filters.ReadLevel(data, level, lineNumbers, maxLines))
	}
	return 0
}

func (a *App) runGrep(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: grep requires a pattern")
		return 2
	}
	pattern := args[0]
	searchPath := "."
	extra := []string{}
	if len(args) > 1 {
		searchPath = args[1]
		if len(args) > 2 {
			extra = args[2:]
		}
	}

	rgArgs := append([]string{"-n", "--no-heading", pattern, searchPath}, extra...)
	cmd := exec.Command("rg", rgArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			fmt.Fprintf(os.Stderr, "szr: %v\n", err)
			return 1
		}
		if exitErr.ExitCode() > 1 {
			fmt.Fprintln(os.Stderr, string(output))
			return exitErr.ExitCode()
		}
	}
	fmt.Println(filters.GroupRipgrep(string(output), a.config.MaxMatchGroups))
	return 0
}

func (a *App) runJSON(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "szr: json requires a file")
		return 2
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Println(filters.RenderJSONStructure(data))
	return 0
}

func (a *App) runLog(args []string) int {
	var data []byte
	var err error
	if len(args) == 0 {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(args[0])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Println(filters.ScannerDedupe(data))
	return 0
}

func (a *App) printHelp() {
	fmt.Print(`szr: "sizer" - token-aware CLI proxy rebuilt in Go

Usage:
  szr git status
  szr go test ./...
  szr grep "pattern" .
  szr read file.go --level aggressive
  szr gain --history
  szr proxy <cmd...>

Core commands:
  git, go, run, test, summary, proxy, explain
  ls, read, grep, json, log
  gain, profiles, doctor
` + "\n")
}
