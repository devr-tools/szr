package cli

import "fmt"

func (a *App) printHelp() {
	fmt.Print(`szr: "sizer" - token-aware CLI proxy rebuilt in Go

Usage:
  szr git status
  szr go test ./...
  szr install codex
  szr bench
  szr grep "pattern" .
  szr read file.go --level aggressive
  szr spread --history
  szr proxy <cmd...>

Core commands:
  git, go, run, test, summary, proxy, explain
  ls, read, grep, json, log
  spread, profiles, doctor, install, bench

Global flags:
  -u, --ultra-compact
  -v, -vv, -vvv, --verbose
  --reasoning-budget <standard|agent>
  --reasoning-budget-mode <standard|agent>
` + "\n")
}
