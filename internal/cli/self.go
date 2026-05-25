package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/selfinstall"
)

func (a *App) runSelf(ctx context.Context, flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: self requires a subcommand")
		return 2
	}

	switch args[0] {
	case "install":
		return a.runSelfInstall(args[1:])
	case "update":
		return a.runSelfUpdate(ctx, args[1:])
	case "doctor":
		return a.runDoctor(ctx, a.configForFlags(flags), args[1:])
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown self subcommand %s\n", args[0])
		return 2
	}
}

func (a *App) runSelfInstall(args []string) int {
	updateShell, printOnly, overrideDir, code := parseSelfInstallArgs(args)
	if code != 0 {
		return code
	}

	plan, err := resolveSelfInstallPlan(overrideDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
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

func parseSelfInstallArgs(args []string) (bool, bool, string, int) {
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
				return false, false, "", 2
			}
			i++
			overrideDir = args[i]
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown self install flag %s\n", args[i])
			return false, false, "", 2
		}
	}
	return updateShell, printOnly, overrideDir, 0
}

func resolveSelfInstallPlan(overrideDir string) (selfinstall.Plan, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return selfinstall.Plan{}, fmt.Errorf("failed to resolve current executable: %w", err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return selfinstall.Plan{}, fmt.Errorf("failed to resolve home directory: %w", err)
	}

	plan, err := selfinstall.PlanInstall(executablePath, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), overrideDir == "")
	if err != nil {
		return selfinstall.Plan{}, err
	}
	if overrideDir == "" {
		return plan, nil
	}
	return selfinstall.WithInstallDir(plan, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), overrideDir), nil
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

func (a *App) runSelfUpdate(ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintf(os.Stderr, "szr: self update does not accept arguments\n")
		return 2
	}
	if a.updater == nil {
		fmt.Fprintln(os.Stderr, "szr: self update is unavailable in this build")
		return 1
	}
	result, err := a.updater.SelfUpdate(ctx, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Printf("updated via: %s\n", result.Method)
	if result.UpgradeCommand != "" {
		fmt.Printf("command: %s\n", result.UpgradeCommand)
	}
	return 0
}

func (a *App) runDoctor(ctx context.Context, cfg config.Config, args []string) int {
	showHistory, code := parseDoctorArgs(args)
	if code != 0 {
		return code
	}

	executablePath, plan, planErr := doctorInstallPlan()

	fmt.Printf("version: %s\n", a.version)
	if executablePath != "" {
		fmt.Printf("executable: %s\n", executablePath)
	}
	printDoctorInstallPlan(plan, planErr)
	fmt.Printf("config: %s\n", a.paths.ConfigFile)
	fmt.Printf("config dir: %s\n", a.paths.ConfigDir)
	fmt.Printf("data dir: %s\n", a.paths.DataDir)
	fmt.Printf("history: %s\n", a.paths.HistoryFile)
	fmt.Printf("tee dir: %s\n", a.paths.TeeDir)
	fmt.Printf("reasoning budget mode: %s\n", cfg.ReasoningBudgetMode)
	a.printDoctorUpdateStatus(ctx, cfg)
	if a.paths.ProjectRuleFile != "" {
		fmt.Printf("project rules: %s\n", a.paths.ProjectRuleFile)
	}
	printDoctorToolStatus()
	if showHistory {
		if err := a.printDoctorHistory(); err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
			return 1
		}
	}
	return 0
}

func parseDoctorArgs(args []string) (bool, int) {
	showHistory := false
	for _, arg := range args {
		switch arg {
		case "--history":
			showHistory = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown doctor flag %s\n", arg)
			return false, 2
		}
	}
	return showHistory, 0
}

func doctorInstallPlan() (string, selfinstall.Plan, error) {
	executablePath, execErr := os.Executable()
	if execErr != nil {
		return "", selfinstall.Plan{}, execErr
	}
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return executablePath, selfinstall.Plan{}, homeErr
	}
	plan, err := selfinstall.PlanInstall(executablePath, homeDir, os.Getenv("PATH"), os.Getenv("SHELL"), false)
	return executablePath, plan, err
}

func printDoctorInstallPlan(plan selfinstall.Plan, err error) {
	if err != nil {
		return
	}
	fmt.Printf("install dir: %s\n", plan.InstallDir)
	fmt.Printf("install target: %s\n", plan.TargetPath)
	if plan.PathContains {
		fmt.Println("path: present")
		return
	}
	fmt.Println("path: missing")
	if plan.ShellRCPath != "" {
		fmt.Printf("shell rc: %s\n", plan.ShellRCPath)
		fmt.Printf("path fix: %s\n", plan.ShellSnippet)
	}
}

func (a *App) printDoctorUpdateStatus(ctx context.Context, cfg config.Config) {
	if a.updater == nil {
		return
	}
	report := a.updater.Doctor(ctx, a.version, cfg.UpdateCheck)
	if !report.Enabled {
		fmt.Println("update checks: disabled")
		fmt.Printf("install method: %s\n", report.Method)
		return
	}
	fmt.Println("update checks: enabled")
	fmt.Printf("update check interval: %s\n", report.Interval)
	fmt.Printf("install method: %s\n", report.Method)
	if !report.CheckedAt.IsZero() {
		fmt.Printf("latest stable: %s\n", report.LatestVersion)
		fmt.Printf("update checked at: %s\n", report.CheckedAt.Format(time.RFC3339))
		if report.UpdateAvailable {
			fmt.Println("update available: yes")
		} else {
			fmt.Println("update available: no")
		}
		if report.UpgradeCommand != "" {
			fmt.Printf("upgrade command: %s\n", report.UpgradeCommand)
		}
	}
	if report.Error != "" {
		fmt.Printf("update check error: %s\n", report.Error)
	}
}

func printDoctorToolStatus() {
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
