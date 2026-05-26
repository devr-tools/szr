package cli

import (
	"context"
	"os"

	workflowcmd "github.com/devr-tools/szr/internal/cli/workflows"
)

type recommendation = workflowcmd.Recommendation
type hotspotStat = workflowcmd.HotspotStat

func (a *App) runRecommend(args []string) int {
	return workflowcmd.RunRecommend(workflowRuntime(a, 0, false), args)
}

func (a *App) runHotspots(args []string) int {
	return workflowcmd.RunHotspots(workflowRuntime(a, 0, false), args)
}

func (a *App) runReplay(flags globalFlags, args []string) int {
	return workflowcmd.RunReplay(workflowRuntime(a, flags.verbose, flags.ultra), args)
}

func (a *App) runCompare(ctx context.Context, flags globalFlags, args []string) int {
	return workflowcmd.RunCompare(ctx, workflowRuntime(a, flags.verbose, flags.ultra), args)
}

func (a *App) runRules(flags globalFlags, args []string) int {
	return workflowcmd.RunRules(workflowRuntime(a, flags.verbose, flags.ultra), args)
}

func (a *App) runScaffold(args []string) int {
	return workflowcmd.RunScaffold(workflowRuntime(a, 0, false), args)
}

func workflowRuntime(a *App, verbose int, ultra bool) workflowcmd.Runtime {
	return workflowcmd.Runtime{
		Config:                a.config,
		Paths:                 a.paths,
		History:               a.history,
		Stdout:                os.Stdout,
		Stderr:                os.Stderr,
		Verbose:               verbose,
		UltraCompact:          ultra,
		DescribeProfileSource: describeProfileSource,
		RelativeToRepo:        relativeToRepo,
	}
}
