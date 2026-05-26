package cli

import (
	"context"
	"os"

	localcmd "github.com/devr-tools/szr/internal/cli/localcmd"
	"github.com/devr-tools/szr/internal/config"
)

func (a *App) runLS(args []string) int {
	return localcmd.RunLS(localRuntime(a), args)
}

func (a *App) runRead(cfg config.Config, args []string) int {
	return localcmd.RunRead(localRuntime(a), cfg, args)
}

func (a *App) runGrep(cfg config.Config, args []string) int {
	return localcmd.RunGrep(localRuntime(a), cfg, args)
}

func (a *App) runFind(cfg config.Config, args []string) int {
	return localcmd.RunFind(localRuntime(a), cfg, args)
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
