package selfcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/devr-tools/szr/internal/selfinstall"
)

func Run(rt Runtime, ctx context.Context, cfg Config, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(rt.Stderr, "szr: self requires a subcommand")
		return 2
	}

	switch args[0] {
	case "install":
		return RunInstall(rt, args[1:])
	case "uninstall":
		return RunUninstall(rt, args[1:])
	case "update":
		return RunUpdate(rt, ctx, args[1:])
	case "doctor":
		return RunDoctor(rt, ctx, cfg, args[1:])
	default:
		fmt.Fprintf(rt.Stderr, "szr: unknown self subcommand %s\n", args[0])
		return 2
	}
}

func RunInstall(rt Runtime, args []string) int {
	updateShell, printOnly, overrideDir, code := parseSelfInstallArgs(rt, args)
	if code != 0 {
		return code
	}

	plan, err := resolveSelfInstallPlan(overrideDir)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	if printOnly {
		printSelfInstallPlan(rt, plan, updateShell)
		return 0
	}

	result, err := selfinstall.Install(plan, updateShell)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to install szr: %v\n", err)
		return 1
	}

	fmt.Fprintf(rt.Stdout, "installed: %s\n", result.Plan.TargetPath)
	fmt.Fprintf(rt.Stdout, "source: %s\n", result.Plan.ExecutablePath)
	if result.Plan.PathContains {
		fmt.Fprintln(rt.Stdout, "path: present")
	} else {
		fmt.Fprintln(rt.Stdout, "path: missing")
	}
	if result.Plan.ShellRCPath != "" {
		fmt.Fprintf(rt.Stdout, "shell rc: %s\n", result.Plan.ShellRCPath)
	}
	if result.ShellUpdated {
		fmt.Fprintln(rt.Stdout, "shell rc updated: yes")
	} else if result.ShellConfigured {
		fmt.Fprintln(rt.Stdout, "shell rc updated: already configured")
	}
	if !result.Plan.PathContains {
		fmt.Fprintln(rt.Stdout, "add this to your shell rc:")
		fmt.Fprintln(rt.Stdout, result.Plan.ShellSnippet)
		if !updateShell {
			fmt.Fprintln(rt.Stdout, "or rerun with: szr self install --update-shell")
		}
	}
	return 0
}

func RunUninstall(rt Runtime, args []string) int {
	printOnly, overrideDir, code := parseSelfUninstallArgs(rt, args)
	if code != 0 {
		return code
	}

	plan, err := resolveSelfInstallPlan(overrideDir)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}

	if printOnly {
		printSelfUninstallPlan(rt, plan)
		return 0
	}

	result, err := selfinstall.Uninstall(plan)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to uninstall szr: %v\n", err)
		return 1
	}

	if result.Removed {
		fmt.Fprintf(rt.Stdout, "uninstalled: %s\n", result.Plan.TargetPath)
	} else {
		fmt.Fprintf(rt.Stdout, "uninstalled: already absent (%s)\n", result.Plan.TargetPath)
	}
	if result.ShellConfigured {
		fmt.Fprintf(rt.Stdout, "shell rc still references install dir: %s\n", result.Plan.ShellRCPath)
	}
	return 0
}

func parseSelfInstallArgs(rt Runtime, args []string) (bool, bool, string, int) {
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
				fmt.Fprintln(rt.Stderr, "szr: self install requires a directory after --path")
				return false, false, "", 2
			}
			i++
			overrideDir = args[i]
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown self install flag %s\n", args[i])
			return false, false, "", 2
		}
	}
	return updateShell, printOnly, overrideDir, 0
}

func parseSelfUninstallArgs(rt Runtime, args []string) (bool, string, int) {
	printOnly := false
	overrideDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--print":
			printOnly = true
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintln(rt.Stderr, "szr: self uninstall requires a directory after --path")
				return false, "", 2
			}
			i++
			overrideDir = args[i]
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown self uninstall flag %s\n", args[i])
			return false, "", 2
		}
	}
	return printOnly, overrideDir, 0
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

func printSelfInstallPlan(rt Runtime, plan selfinstall.Plan, updateShell bool) {
	fmt.Fprintln(rt.Stdout, "plan: self install")
	fmt.Fprintf(rt.Stdout, "  target: %s\n", plan.TargetPath)
	fmt.Fprintf(rt.Stdout, "  source: %s\n", plan.ExecutablePath)
	if plan.PathContains {
		fmt.Fprintln(rt.Stdout, "  path: present")
	} else {
		fmt.Fprintln(rt.Stdout, "  path: missing")
	}
	if plan.ShellRCPath != "" {
		fmt.Fprintf(rt.Stdout, "  shell rc: %s\n", plan.ShellRCPath)
	}
	if !plan.PathContains {
		fmt.Fprintln(rt.Stdout, "  shell snippet:")
		fmt.Fprintf(rt.Stdout, "    %s\n", plan.ShellSnippet)
	}
	if updateShell {
		fmt.Fprintln(rt.Stdout, "  update shell rc: yes")
	}
}

func printSelfUninstallPlan(rt Runtime, plan selfinstall.Plan) {
	fmt.Fprintln(rt.Stdout, "plan: self uninstall")
	fmt.Fprintf(rt.Stdout, "  target: %s\n", plan.TargetPath)
	if plan.ShellRCPath != "" {
		fmt.Fprintf(rt.Stdout, "  shell rc: %s\n", plan.ShellRCPath)
	}
	if plan.ShellSnippet != "" {
		fmt.Fprintf(rt.Stdout, "  path snippet: %s\n", plan.ShellSnippet)
	}
}

func RunUpdate(rt Runtime, ctx context.Context, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(rt.Stderr, "szr: self update does not accept arguments")
		return 2
	}
	if rt.SelfUpdate == nil {
		fmt.Fprintln(rt.Stderr, "szr: self update is unavailable in this build")
		return 1
	}
	stdout, _ := rt.Stdout.(*os.File)
	stderr, _ := rt.Stderr.(*os.File)
	if stdout == nil || stderr == nil {
		fmt.Fprintln(rt.Stderr, "szr: self update requires file-backed stdio")
		return 1
	}
	result, err := rt.SelfUpdate(ctx, stdout, stderr)
	if err != nil {
		if err == ErrSelfUpdateUnavailable {
			fmt.Fprintln(rt.Stderr, "szr: self update is unavailable in this build")
			return 1
		}
		fmt.Fprintf(rt.Stderr, "szr: %v\n", err)
		return 1
	}
	fmt.Fprintf(rt.Stdout, "updated via: %s\n", result.Method)
	if result.UpgradeCommand != "" {
		fmt.Fprintf(rt.Stdout, "command: %s\n", result.UpgradeCommand)
	}
	return 0
}
