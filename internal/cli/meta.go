package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"szr/internal/history"
)

func (a *App) runSpread(args []string) int {
	showHistory := false
	asJSON := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		case "--json":
			asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown spread flag %s\n", arg)
			return 2
		}
	}

	records, err := a.history.LoadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}
	summary := history.Summarize(records, 8)
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
		return 0
	}

	if summary.Commands == 0 {
		fmt.Println("no tracked commands yet")
		return 0
	}

	fmt.Printf("commands: %d\n", summary.Commands)
	fmt.Printf("avg savings: %.1f%%\n", summary.AveragePct)
	fmt.Printf("tokens saved: %d\n", summary.SavedTokens)
	fmt.Printf("failures: %d\n", summary.Failures)
	if len(summary.TopCommands) > 0 {
		fmt.Println("top commands:")
		for _, cmd := range summary.TopCommands {
			fmt.Printf("  %s (%d)\n", cmd.Command, cmd.Count)
		}
	}
	if showHistory {
		fmt.Println("recent:")
		for _, rec := range summary.Recent {
			fmt.Printf("  %s  %s  %s  %.1f%%\n", rec.Timestamp.Format(time.RFC3339), rec.Profile, rec.Command, rec.SavingsPct)
		}
	}
	return 0
}

func (a *App) runProfiles() int {
	for _, profile := range a.engine.Profiles() {
		fmt.Printf("%s\n  %s\n", profile.Name, profile.Description)
	}
	return 0
}

func (a *App) runDoctor() int {
	fmt.Printf("version: %s\n", a.version)
	fmt.Printf("config: %s\n", a.paths.ConfigFile)
	fmt.Printf("history: %s\n", a.paths.HistoryFile)
	fmt.Printf("tee dir: %s\n", a.paths.TeeDir)
	for _, tool := range []string{"git", "go", "rg"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Printf("%s: missing\n", tool)
			continue
		}
		fmt.Printf("%s: %s\n", tool, path)
	}
	return 0
}
