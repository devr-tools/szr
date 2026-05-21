package cli

import (
	"fmt"
	"os"
	"os/exec"

	"szr/internal/config"
	"szr/internal/history"
	"szr/internal/selfinstall"
)

func (a *App) runSelf(flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: self requires a subcommand")
		return 2
	}

	switch args[0] {
	case "install":
		return a.runSelfInstall(args[1:])
	case "doctor":
		return a.runDoctor(a.configForFlags(flags), args[1:])
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown self subcommand %s\n", args[0])
		return 2
	}
}

func (a *App) runSelfInstall(args []string) int {
	updateShell := false
	printOnly := false
	overrideDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--update-shell":
			updateShell = true
		case "--print":
			printOnly = true
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: self install requires a directory after --path")
				return 2
			}
			i++
			overrideDir = args[i]
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown self install flag %s\n", args[i])
			return 2
		}
	}

	executablePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to resolve current executable: %v\n", err)
		return 1
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to resolve home directory: %v\n", err)
		return 1
	}

	plan, err := selfinstall.PlanInstall(executablePath, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), overrideDir == "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	if overrideDir != "" {
		plan = selfinstall.WithInstallDir(plan, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), overrideDir)
	}

	if printOnly {
		printSelfInstallPlan(plan, updateShell)
		return 0
	}

	result, err := selfinstall.Install(plan, updateShell)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to install szr: %v\n", err)
		return 1
	}

	fmt.Printf("installed: %s\n", result.Plan.TargetPath)
	fmt.Printf("source: %s\n", result.Plan.ExecutablePath)
	if result.Plan.PathContains {
		fmt.Println("path: present")
	} else {
		fmt.Println("path: missing")
	}
	if result.Plan.ShellRCPath != "" {
		fmt.Printf("shell rc: %s\n", result.Plan.ShellRCPath)
	}
	if result.ShellUpdated {
		fmt.Println("shell rc updated: yes")
	} else if result.ShellConfigured {
		fmt.Println("shell rc updated: already configured")
	}
	if !result.Plan.PathContains {
		fmt.Println("add this to your shell rc:")
		fmt.Println(result.Plan.ShellSnippet)
		if !updateShell {
			fmt.Println("or rerun with: szr self install --update-shell")
		}
	}
	return 0
}

func printSelfInstallPlan(plan selfinstall.Plan, updateShell bool) {
	fmt.Println("plan: self install")
	fmt.Printf("  target: %s\n", plan.TargetPath)
	fmt.Printf("  source: %s\n", plan.ExecutablePath)
	if plan.PathContains {
		fmt.Println("  path: present")
	} else {
		fmt.Println("  path: missing")
	}
	if plan.ShellRCPath != "" {
		fmt.Printf("  shell rc: %s\n", plan.ShellRCPath)
	}
	if !plan.PathContains {
		fmt.Println("  shell snippet:")
		fmt.Printf("    %s\n", plan.ShellSnippet)
	}
	if updateShell {
		fmt.Println("  update shell rc: yes")
	}
}

func (a *App) runDoctor(cfg config.Config, args []string) int {
	showHistory := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown doctor flag %s\n", arg)
			return 2
		}
	}

	executablePath, execErr := os.Executable()
	homeDir, homeErr := os.UserHomeDir()
	var plan selfinstall.Plan
	var planErr error
	if execErr == nil && homeErr == nil {
		plan, planErr = selfinstall.PlanInstall(executablePath, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), false)
	}

	fmt.Printf("version: %s\n", a.version)
	if execErr == nil {
		fmt.Printf("executable: %s\n", executablePath)
	}
	if planErr == nil {
		fmt.Printf("install dir: %s\n", plan.InstallDir)
		fmt.Printf("install target: %s\n", plan.TargetPath)
		if plan.PathContains {
			fmt.Println("path: present")
		} else {
			fmt.Println("path: missing")
			if plan.ShellRCPath != "" {
				fmt.Printf("shell rc: %s\n", plan.ShellRCPath)
				fmt.Printf("path fix: %s\n", plan.ShellSnippet)
			}
		}
	}
	fmt.Printf("config: %s\n", a.paths.ConfigFile)
	fmt.Printf("config dir: %s\n", a.paths.ConfigDir)
	fmt.Printf("data dir: %s\n", a.paths.DataDir)
	fmt.Printf("history: %s\n", a.paths.HistoryFile)
	fmt.Printf("tee dir: %s\n", a.paths.TeeDir)
	fmt.Printf("reasoning budget mode: %s\n", cfg.ReasoningBudgetMode)
	if a.paths.ProjectRuleFile != "" {
		fmt.Printf("project rules: %s\n", a.paths.ProjectRuleFile)
	}
	for _, tool := range []string{"git", "go"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			fmt.Printf("%s: missing\n", tool)
			continue
		}
		fmt.Printf("%s: %s\n", tool, path)
	}
	if path, err := exec.LookPath("rg"); err != nil {
		fmt.Println("rg: missing (optional; only needed for `szr rg`)")
	} else {
		fmt.Printf("rg: %s (optional)\n", path)
	}
	if showHistory {
		if err := a.printDoctorHistory(); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
			return 1
		}
	}
	return 0
}

func (a *App) printDoctorHistory() error {
	records, err := a.history.LoadAll()
	if err != nil {
		return err
	}
	summary := history.Summarize(records, 5)
	fmt.Println("history diagnostics:")
	fmt.Printf("  commands: %d\n", summary.Commands)
	fmt.Printf("  fallback rate: %.1f%%\n", summary.FallbackRate)
	fmt.Printf("  failure rate: %.1f%%\n", summary.FailureRate)
	fmt.Printf("  tee rate: %.1f%%\n", summary.TeeRate)
	recommendations := buildRecommendations(records, 3)
	if len(recommendations) > 0 {
		fmt.Println("  recommendations:")
		for _, item := range recommendations {
			fmt.Printf("    - [%s] %s\n", item.Kind, item.Action)
		}
	}
	hotspots := buildHotspots(records, 3)
	if len(hotspots) > 0 {
		fmt.Println("  hotspots:")
		for _, item := range hotspots {
			fmt.Printf("    - %s profile=%s avg=%.1f%% fallback=%.1f%% p95=%dms\n", item.Command, item.Profile, item.AveragePct, item.FallbackRate, item.DurationP95MS)
		}
	}
	return nil
}
