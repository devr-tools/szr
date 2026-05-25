package szr

import (
	"context"

	"github.com/devr-tools/szr/internal/cli"
)

// App exposes the public szr entrypoint without leaking internal CLI wiring.
type App struct {
	cli *cli.App
}

func New(version string) *App {
	return &App{cli: cli.New(version)}
}

func NewWithCLI(app *cli.App) *App {
	return &App{cli: app}
}

func Run(ctx context.Context, version string, args []string) int {
	return New(version).Run(ctx, args)
}

func (a *App) Run(ctx context.Context, args []string) int {
	return a.cli.Run(ctx, args)
}
