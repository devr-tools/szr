package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/devr-tools/szr/internal/budgethints"
)

func (a *App) runGateway(ctx context.Context, args []string) int {
	if !gatewayRefreshRequested(args) {
		fmt.Fprintln(os.Stderr, "szr: gateway requires hints-refresh")
		return 2
	}
	client, err := a.gatewayHintClient()
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

func gatewayRefreshRequested(args []string) bool { return len(args) == 1 && args[0] == "hints-refresh" }

func (a *App) gatewayHintClient() (*budgethints.Client, error) {
	cfg := a.config.GatewayHints
	if !cfg.Enabled {
		return nil, fmt.Errorf("gateway hints are disabled")
	}
	return budgethints.NewClient(budgethints.ClientConfig{
		Endpoint: cfg.Endpoint, BearerToken: os.Getenv(cfg.AuthTokenEnv), SigningPublicKey: cfg.SigningPublicKey,
		Store: budgethints.New(filepath.Join(a.paths.DataDir, "gateway-budget-hints.json")),
	})
}
