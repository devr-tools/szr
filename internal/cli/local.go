package cli

import (
	"context"
	"os"

	localcmd "github.com/devr-tools/szr/internal/cli/localcmd"
	"github.com/devr-tools/szr/internal/config"
)

func (a *App) runLS(ctx context.Context, flags globalFlags, args []string) int {
	if code, handled := localcmd.TryRunLS(localRuntime(a), args); handled {
		return code
	}
	return a.runNativeFallback(ctx, flags, "ls", args)
}

func (a *App) runRead(cfg config.Config, args []string) int {
	return localcmd.RunRead(localRuntime(a), cfg, args)
}

func (a *App) runGrep(ctx context.Context, flags globalFlags, args []string) int {
	if code, handled := localcmd.TryRunGrep(localRuntime(a), a.configForFlags(flags), args); handled {
		return code
	}
	return a.runNativeFallback(ctx, flags, "grep", args)
}

func (a *App) runFind(ctx context.Context, flags globalFlags, args []string) int {
	if code, handled := localcmd.TryRunFind(localRuntime(a), a.configForFlags(flags), args); handled {
		return code
	}
	return a.runNativeFallback(ctx, flags, "find", args)
}

// runNativeFallback executes the command the user actually typed through the
// engine, so profile filtering still applies and the caller sees the native
// binary's exit code.
func (a *App) runNativeFallback(ctx context.Context, flags globalFlags, name string, args []string) int {
	return a.runExternal(ctx, flags, "run", append([]string{name}, args...), false)
}

func (a *App) runRGExternal(ctx context.Context, flags globalFlags, args []string) int {
	return localcmd.RunRGExternal(localRuntime(a), func(args []string) int {
		return a.runExternal(ctx, flags, "run", append([]string{"rg"}, args...), false)
	}, args)
}

func (a *App) runJSON(args []string) int {
	return localcmd.RunJSON(localRuntime(a), args)
}

func (a *App) runLog(args []string) int {
	return localcmd.RunLog(localRuntime(a), args)
}

func localRuntime(a *App) localcmd.Runtime {
	return localcmd.Runtime{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}
