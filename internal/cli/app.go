package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
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
	cfg.ReasoningBudgetMode = config.ResolveReasoningBudgetMode(cfg.ReasoningBudgetMode)
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
	flags, rest, err := parseGlobalFlags(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 2
	}
	if len(rest) == 0 {
		a.printHelp()
		return 0
	}

	if code, ok := a.runBuiltInCommand(ctx, flags, rest); ok {
		return code
	}
	return a.runExternal(ctx, flags, "run", rest, false)
}

func (a *App) runBuiltInCommand(ctx context.Context, flags globalFlags, rest []string) (int, bool) {
	command := rest[0]
	handlers := map[string]func() int{
		"--help":    func() int { a.printHelp(); return 0 },
		"-h":        func() int { a.printHelp(); return 0 },
		"help":      func() int { a.printHelp(); return 0 },
		"commands":  func() int { a.printCommands(); return 0 },
		"--version": func() int { fmt.Println("szr", a.version); return 0 },
		"version":   func() int { fmt.Println("szr", a.version); return 0 },
		"spread":    func() int { return a.runSpread(rest[1:]) },
		"gain":      func() int { return a.runSpread(rest[1:]) },
		"recommend": func() int { return a.runRecommend(rest[1:]) },
		"hotspots":  func() int { return a.runHotspots(rest[1:]) },
		"profiles":  func() int { return a.runProfiles() },
		"doctor":    func() int { return a.runDoctor(a.configForFlags(flags), rest[1:]) },
		"self":      func() int { return a.runSelf(flags, rest[1:]) },
		"install":   func() int { return a.runInstall(rest[1:]) },
		"bench":     func() int { return a.runBench(rest[1:]) },
		"replay":    func() int { return a.runReplay(flags, rest[1:]) },
		"compare":   func() int { return a.runCompare(ctx, flags, rest[1:]) },
		"rules":     func() int { return a.runRules(flags, rest[1:]) },
		"scaffold":  func() int { return a.runScaffold(rest[1:]) },
		"proxy":     func() int { return a.runExternal(ctx, flags, "proxy", rest[1:], true) },
		"run":       func() int { return a.runExternal(ctx, flags, "run", rest[1:], false) },
		"test":      func() int { return a.runExternal(ctx, flags, "test", rest[1:], false) },
		"summary":   func() int { return a.runExternal(ctx, flags, "summary", rest[1:], false) },
		"explain":   func() int { return a.runExplain(flags, rest[1:]) },
		"git":       func() int { return a.runExternal(ctx, flags, command, rest[1:], false) },
		"go":        func() int { return a.runExternal(ctx, flags, command, rest[1:], false) },
		"ls":        func() int { return a.runLS(rest[1:]) },
		"read":      func() int { return a.runRead(a.configForFlags(flags), rest[1:]) },
		"grep":      func() int { return a.runGrep(a.configForFlags(flags), rest[1:]) },
		"rg":        func() int { return a.runRGExternal(ctx, flags, rest[1:]) },
		"json":      func() int { return a.runJSON(rest[1:]) },
		"log":       func() int { return a.runLog(rest[1:]) },
		"tee":       func() int { return a.runTee(rest[1:]) },
	}
	handler, ok := handlers[command]
	if !ok {
		return 0, false
	}
	return handler(), true
}

func (a *App) configForFlags(flags globalFlags) config.Config {
	cfg := a.config
	if flags.reasoningBudgetSet {
		cfg.ReasoningBudgetMode = flags.reasoningBudget
	}
	cfg.ReasoningBudgetMode = config.ResolveReasoningBudgetMode(cfg.ReasoningBudgetMode)
	return cfg
}

func (a *App) engineForFlags(flags globalFlags) *engine.Engine {
	if !flags.reasoningBudgetSet {
		return a.engine
	}
	cfg := a.configForFlags(flags)
	return engine.New(cfg, a.paths, a.history, profiles.Builtins(cfg.MaxPreviewLines))
}
