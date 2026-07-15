package cli

import (
	"context"
	"os"

	selfcmd "github.com/devr-tools/szr/internal/cli/selfcmd"
	"github.com/devr-tools/szr/internal/config"
)

func (a *App) runSelf(ctx context.Context, flags globalFlags, args []string) int {
	return selfcmd.Run(selfRuntime(a), ctx, a.configForFlags(flags), args)
}

func (a *App) runSelfInstall(args []string) int {
	return selfcmd.RunInstall(selfRuntime(a), args)
}

func (a *App) runSelfUninstall(args []string) int {
	return selfcmd.RunUninstall(selfRuntime(a), args)
}

func (a *App) runSelfUpdate(ctx context.Context, args []string) int {
	return selfcmd.RunUpdate(selfRuntime(a), ctx, args)
}

func (a *App) runDoctor(ctx context.Context, cfg config.Config, args []string) int {
	return selfcmd.RunDoctor(selfRuntime(a), ctx, cfg, args)
}

func selfRuntime(a *App) selfcmd.Runtime {
	return selfcmd.Runtime{
		Version: a.version,
		Config:  a.config,
		Paths:   a.paths,
		History: a.history,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		SelfUpdate: func(ctx context.Context, stdout, stderr *os.File) (selfcmd.SelfUpdateResult, error) {
			if a.updater == nil {
				return selfcmd.SelfUpdateResult{}, selfcmd.ErrSelfUpdateUnavailable
			}
			result, err := a.updater.SelfUpdate(ctx, stdout, stderr)
			return selfcmd.SelfUpdateResult{
				Method:         result.Method,
				UpgradeCommand: result.UpgradeCommand,
			}, err
		},
		DoctorReport: func(ctx context.Context, cfg selfcmd.Config, opts ...selfcmd.DoctorOption) selfcmd.DoctorReport {
			if a.updater == nil {
				return selfcmd.DoctorReport{}
			}
			report := a.updater.Doctor(ctx, a.version, cfg.UpdateCheck, opts...)
			return selfcmd.DoctorReport(report)
		},
	}
}
