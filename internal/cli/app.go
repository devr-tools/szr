package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/internal/updates"
)

type App struct {
	version string
	config  config.Config
	paths   config.Paths
	history *history.Store
	engine  *engine.Engine
	updater updater
}

type updater interface {
	Doctor(context.Context, string, config.UpdateCheck) updates.DoctorReport
	DoctorWithOptions(context.Context, string, config.UpdateCheck, ...updates.DoctorOption) updates.DoctorReport
	AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult
	SelfUpdate(context.Context, io.Writer, io.Writer) (updates.SelfUpdateResult, error)
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
	return NewWithDependenciesAndUpdater(version, cfg, paths, store, eng, updates.New(paths))
}

func NewWithDependenciesAndUpdater(
	version string,
	cfg config.Config,
	paths config.Paths,
	store *history.Store,
	eng *engine.Engine,
	updater updater,
) *App {
	cfg.ReasoningBudgetMode = config.ResolveReasoningBudgetMode(cfg.ReasoningBudgetMode)
	return &App{
		version: version,
		config:  cfg,
		paths:   paths,
		history: store,
		engine:  eng,
		updater: updater,
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
	return NewWithDependenciesAndUpdater(
		version,
		cfg,
		paths,
		store,
		engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines)),
		updates.New(paths),
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

	finishUpdateFlow := a.startUpdateFlow(ctx, rest)
	defer finishUpdateFlow()

	if code, ok := a.runBuiltInCommand(ctx, flags, rest); ok {
		return code
	}
	return a.runExternal(ctx, flags, "run", rest, false)
}

// updateFlowJoinTimeout bounds how long a finished command waits for the
// background update probe. A probe still in flight defers its notice to a
// later invocation instead of holding the process open.
const updateFlowJoinTimeout = 2 * time.Second

// startUpdateFlow moves the update check off the dispatch path. The doctor
// probe — the only step that can hit the network — warms the release cache
// in the background while the command runs; auto-update and the interactive
// notice run after the command completes against the warmed cache, so their
// output never interleaves with the command's.
func (a *App) startUpdateFlow(ctx context.Context, rest []string) func() {
	if !a.updateFlowWanted(rest) {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.updater.Doctor(ctx, a.version, a.config.UpdateCheck)
	}()
	return func() {
		select {
		case <-done:
		case <-time.After(updateFlowJoinTimeout):
			return
		}
		autoResult := a.maybeAutoUpdate(ctx, rest)
		a.maybePrintUpdateNotice(ctx, rest, autoResult)
	}
}

// updateFlowWanted mirrors the gates of maybeAutoUpdate and
// maybePrintUpdateNotice so the background probe never performs update
// checks the previous inline flow would not have made.
func (a *App) updateFlowWanted(rest []string) bool {
	if a.updater == nil || !a.config.UpdateCheck.Enabled || len(rest) == 0 || isUpdateExemptCommand(rest) {
		return false
	}
	return a.config.UpdateCheck.AutoUpdate || isInteractiveFile(os.Stderr)
}

func isUpdateExemptCommand(rest []string) bool {
	return rest[0] == "doctor" || (rest[0] == "self" && len(rest) > 1 && (rest[1] == "doctor" || rest[1] == "update"))
}

func (a *App) maybeAutoUpdate(ctx context.Context, rest []string) updates.AutoUpdateResult {
	if a.updater == nil || len(rest) == 0 || isUpdateExemptCommand(rest) {
		return updates.AutoUpdateResult{}
	}
	return a.updater.AutoUpdate(ctx, a.version, a.config.UpdateCheck, os.Stdout, os.Stderr)
}

func (a *App) maybePrintUpdateNotice(ctx context.Context, rest []string, autoResult updates.AutoUpdateResult) {
	if a.updater == nil || !a.config.UpdateCheck.Enabled || len(rest) == 0 || !isInteractiveFile(os.Stderr) {
		return
	}
	if isUpdateExemptCommand(rest) {
		return
	}
	if autoResult.Updated {
		return
	}
	report := a.updater.Doctor(ctx, a.version, a.config.UpdateCheck)
	if !report.Enabled || !report.UpdateAvailable || report.UpgradeCommand == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "szr: update available: %s (current %s). Run: %s\n", report.LatestVersion, a.version, report.UpgradeCommand)
}

func isInteractiveFile(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (a *App) runBuiltInCommand(ctx context.Context, flags globalFlags, rest []string) (int, bool) {
	command := rest[0]
	handlers := map[string]func() int{
		"--help":        func() int { a.printHelp(); return 0 },
		"-h":            func() int { a.printHelp(); return 0 },
		"help":          func() int { a.printHelp(); return 0 },
		"commands":      func() int { a.printCommands(); return 0 },
		"--version":     func() int { fmt.Println("szr", a.version); return 0 },
		"version":       func() int { fmt.Println("szr", a.version); return 0 },
		"spread":        func() int { return a.runSpread(rest[1:]) },
		"clear-spread":  func() int { return a.runClearSpread(rest[1:]) },
		"reset-history": func() int { return a.runResetHistory(rest[1:]) },
		"gain":          func() int { return a.runSpread(rest[1:]) },
		"discover":      func() int { return a.runDiscover(rest[1:]) },
		"usage":         func() int { return a.runUsage(rest[1:]) },
		"recommend":     func() int { return a.runRecommend(rest[1:]) },
		"hotspots":      func() int { return a.runHotspots(rest[1:]) },
		"profiles":      func() int { return a.runProfiles() },
		"settings":      func() int { return a.runSettings(rest[1:]) },
		"doctor":        func() int { return a.runDoctor(ctx, a.configForFlags(flags), rest[1:]) },
		"self":          func() int { return a.runSelf(ctx, flags, rest[1:]) },
		"install":       func() int { return a.runInstall(rest[1:]) },
		"uninstall":     func() int { return a.runUninstall(rest[1:]) },
		"bench":         func() int { return a.runBench(rest[1:]) },
		"replay":        func() int { return a.runReplay(flags, rest[1:]) },
		"compare":       func() int { return a.runCompare(ctx, flags, rest[1:]) },
		"rewrite":       func() int { return a.runRewrite(rest[1:]) },
		"rules":         func() int { return a.runRules(flags, rest[1:]) },
		"scaffold":      func() int { return a.runScaffold(rest[1:]) },
		"proxy":         func() int { return a.runExternal(ctx, flags, "proxy", rest[1:], true) },
		"run":           func() int { return a.runExternal(ctx, flags, "run", rest[1:], false) },
		"test":          func() int { return a.runExternal(ctx, flags, "test", rest[1:], false) },
		"summary":       func() int { return a.runExternal(ctx, flags, "summary", rest[1:], false) },
		"explain":       func() int { return a.runExplain(flags, rest[1:]) },
		"git":           func() int { return a.runExternal(ctx, flags, command, rest[1:], false) },
		"go":            func() int { return a.runExternal(ctx, flags, command, rest[1:], false) },
		"ls":            func() int { return a.runLS(ctx, flags, rest[1:]) },
		"find":          func() int { return a.runFind(ctx, flags, rest[1:]) },
		"read":          func() int { return a.runRead(a.configForFlags(flags), rest[1:]) },
		"grep":          func() int { return a.runGrep(ctx, flags, rest[1:]) },
		"rg":            func() int { return a.runRGExternal(ctx, flags, rest[1:]) },
		"json":          func() int { return a.runJSON(rest[1:]) },
		"log":           func() int { return a.runLog(rest[1:]) },
		"pipe":          func() int { return a.runPipe(a.configForFlags(flags), rest[1:]) },
		"tee":           func() int { return a.runTee(rest[1:]) },
		"expand":        func() int { return a.runExpand(rest[1:]) },
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
