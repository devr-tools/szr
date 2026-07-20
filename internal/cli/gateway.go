package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devr-tools/szr/internal/budgethints"
)

func (a *App) runGateway(ctx context.Context, args []string) int {
	if len(args) != 1 || args[0] != "hints-refresh" {
		fmt.Fprintln(os.Stderr, "szr: gateway requires hints-refresh")
		return 2
	}
	cfg := a.config.GatewayHints
	if !cfg.Enabled {
		fmt.Fprintln(os.Stderr, "szr: gateway hints are disabled")
		return 2
	}
	token := os.Getenv(cfg.AuthTokenEnv)
	client, err := budgethints.NewClient(budgethints.ClientConfig{
		Endpoint: cfg.Endpoint, BearerToken: token, SigningPublicKey: cfg.SigningPublicKey,
		Store: budgethints.New(filepath.Join(a.paths.DataDir, "gateway-budget-hints.json")),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: gateway hints: %v\n", err)
		return 1
	}
	count, err := client.Refresh(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: gateway hints refresh: %v\n", err)
		return 1
	}
	fmt.Printf("refreshed gateway budget hints: %d\n", count)
	return 0
}
