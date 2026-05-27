package cli

import (
	"fmt"
	"os"
)

func (a *App) runClearSpread(args []string) int {
	return a.runResetHistoryCommand("clear-spread", args)
}

func (a *App) runResetHistory(args []string) int {
	return a.runResetHistoryCommand("reset-history", args)
}

func (a *App) runResetHistoryCommand(name string, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "szr: %s does not accept arguments\n", name)
		return 2
	}
	if err := a.history.Clear(); err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to clear spread history: %v\n", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, "cleared spread history")
	return 0
}
