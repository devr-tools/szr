package cli

import (
	"context"
	"fmt"
	"os"

	"szr/internal/config"
	"szr/internal/engine"
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

func New(version string) *App {
	return NewWithLoader(version, config.Load, os.Exit)
}

func NewWithDependencies(
	version string,
	cfg config.Config,
	paths config.Paths,
	store *history.Store,
	eng *engine.Engine,
) *App {
	return &App{
		version: version,
		config:  cfg,
		paths:   paths,
		history: store,
		engine:  eng,
	}
}

func NewWithLoader(
	version string,
	load func() (config.Config, config.Paths, error),
	exit func(int),
) *App {
	cfg, paths, err := load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to load config: %v\n", err)
		exit(1)
		return nil
	}

	store := history.New(paths.HistoryFile)
	return NewWithDependencies(
		version,
		cfg,
		paths,
		store,
		engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines)),
	)
}

func (a *App) Run(ctx context.Context, args []string) int {
	flags, rest := parseGlobalFlags(args)
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
