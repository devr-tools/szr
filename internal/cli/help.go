package cli

import "fmt"

func (a *App) printHelp() {
	fmt.Print(`szr: "sizer" - token-aware CLI proxy rebuilt in Go

Usage:
  szr git status
  szr go test ./...
  szr grep "pattern" .
  szr read file.go --level aggressive
  szr gain --history
  szr proxy <cmd...>

Core commands:
  git, go, run, test, summary, proxy, explain
  ls, read, grep, json, log
  gain, profiles, doctor
` + "\n")
}
