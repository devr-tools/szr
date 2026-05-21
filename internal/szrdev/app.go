package szrdev

import (
	"context"

	szrpkg "szr/pkg/szr"
)

const version = "dev"

func New() *szrpkg.App {
	return szrpkg.New(version)
}

func Run(ctx context.Context, args []string) int {
	return New().Run(ctx, args)
}
