package selfcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	workflowcmd "github.com/devr-tools/szr/internal/cli/workflows"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/selfinstall"
)

func RunDoctor(rt Runtime, ctx context.Context, cfg Config, args []string) int {
	parsed, code := parseDoctorArgs(rt, args)
	if code != 0 {
		return code
	}

	executablePath, plan, planErr := doctorInstallPlan()
	update := doctorUpdateReport(rt, ctx, cfg)
	historyDiagnostics, err := doctorHistoryDiagnostics(rt, parsed.showHistory)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}

	if parsed.asJSON {
		payload := doctorJSON{
			Version:             rt.Version,
			Executable:          executablePath,
			Config:              rt.Paths.ConfigFile,
			ConfigDir:           rt.Paths.ConfigDir,
			DataDir:             rt.Paths.DataDir,
			History:             rt.Paths.HistoryFile,
			TeeDir:              rt.Paths.TeeDir,
			ReasoningBudgetMode: cfg.ReasoningBudgetMode,
			Update:              doctorUpdateJSONFromReport(update),
			Tools:               doctorToolStatusJSON(),
			HistoryDiagnostics:  historyDiagnostics,
		}
		if planErr == nil {
			payload.InstallDir = plan.InstallDir
			payload.InstallTarget = plan.TargetPath
			payload.PathPresent = plan.PathContains
			payload.ShellRC = plan.ShellRCPath
			payload.PathFix = plan.ShellSnippet
		}
		if rt.Paths.ProjectRuleFile != "" {
			payload.ProjectRules = rt.Paths.ProjectRuleFile
		}
		enc := json.NewEncoder(rt.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

	fmt.Fprintf(rt.Stdout, "version: %s\n", rt.Version)
	if executablePath != "" {
		fmt.Fprintf(rt.Stdout, "executable: %s\n", executablePath)
	}
	printDoctorInstallPlan(rt, plan, planErr)
	fmt.Fprintf(rt.Stdout, "config: %s\n", rt.Paths.ConfigFile)
	fmt.Fprintf(rt.Stdout, "config dir: %s\n", rt.Paths.ConfigDir)
	fmt.Fprintf(rt.Stdout, "data dir: %s\n", rt.Paths.DataDir)
	fmt.Fprintf(rt.Stdout, "history: %s\n", rt.Paths.HistoryFile)
	fmt.Fprintf(rt.Stdout, "tee dir: %s\n", rt.Paths.TeeDir)
	fmt.Fprintf(rt.Stdout, "reasoning budget mode: %s\n", cfg.ReasoningBudgetMode)
	printDoctorUpdateStatus(rt, update)
	if rt.Paths.ProjectRuleFile != "" {
		fmt.Fprintf(rt.Stdout, "project rules: %s\n", rt.Paths.ProjectRuleFile)
	}
	printDoctorToolStatus(rt)
	if parsed.showHistory && historyDiagnostics != nil {
		printDoctorHistory(rt, historyDiagnostics)
	}
	return 0
}

func parseDoctorArgs(rt Runtime, args []string) (doctorArgs, int) {
	parsed := doctorArgs{}
	for _, arg := range args {
		switch arg {
		case "--history":
			parsed.showHistory = true
		case "--json":
			parsed.asJSON = true
		default:
			fmt.Fprintf(rt.Stderr, "szr: unknown doctor flag %s\n", arg)
			return doctorArgs{}, 2
		}
	}
	return parsed, 0
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

func printDoctorInstallPlan(rt Runtime, plan selfinstall.Plan, err error) {
	if err != nil {
		return
	}
	fmt.Fprintf(rt.Stdout, "install dir: %s\n", plan.InstallDir)
	fmt.Fprintf(rt.Stdout, "install target: %s\n", plan.TargetPath)
	if plan.PathContains {
		fmt.Fprintln(rt.Stdout, "path: present")
		return
	}
	fmt.Fprintln(rt.Stdout, "path: missing")
	if plan.ShellRCPath != "" {
		fmt.Fprintf(rt.Stdout, "shell rc: %s\n", plan.ShellRCPath)
		fmt.Fprintf(rt.Stdout, "path fix: %s\n", plan.ShellSnippet)
	}
}

func doctorUpdateReport(rt Runtime, ctx context.Context, cfg Config) DoctorReport {
	if rt.DoctorReport == nil {
		return DoctorReport{}
	}
	return rt.DoctorReport(ctx, cfg)
}

func printDoctorUpdateStatus(rt Runtime, report DoctorReport) {
	if !report.Enabled {
		fmt.Fprintln(rt.Stdout, "update checks: disabled")
		fmt.Fprintf(rt.Stdout, "install method: %s\n", report.Method)
		if report.AutoUpdate {
			fmt.Fprintln(rt.Stdout, "auto update: enabled")
		} else {
			fmt.Fprintln(rt.Stdout, "auto update: disabled")
		}
		return
	}
	fmt.Fprintln(rt.Stdout, "update checks: enabled")
	fmt.Fprintf(rt.Stdout, "update check interval: %s\n", report.Interval)
	if report.AutoUpdate {
		fmt.Fprintln(rt.Stdout, "auto update: enabled")
	} else {
		fmt.Fprintln(rt.Stdout, "auto update: disabled")
	}
	fmt.Fprintf(rt.Stdout, "install method: %s\n", report.Method)
	if !report.CheckedAt.IsZero() {
		fmt.Fprintf(rt.Stdout, "latest stable: %s\n", report.LatestVersion)
		fmt.Fprintf(rt.Stdout, "update checked at: %s\n", report.CheckedAt.Format(time.RFC3339))
		if report.UpdateAvailable {
			fmt.Fprintln(rt.Stdout, "update available: yes")
		} else {
			fmt.Fprintln(rt.Stdout, "update available: no")
		}
		if report.UpgradeCommand != "" {
			fmt.Fprintf(rt.Stdout, "upgrade command: %s\n", report.UpgradeCommand)
		}
	}
	if !report.AutoUpdateState.SucceededAt.IsZero() {
		fmt.Fprintf(rt.Stdout, "last auto update: %s %s\n", report.AutoUpdateState.SucceededVersion, report.AutoUpdateState.SucceededAt.Format(time.RFC3339))
	} else if !report.AutoUpdateState.AttemptedAt.IsZero() {
		fmt.Fprintf(rt.Stdout, "last auto update attempt: %s %s\n", report.AutoUpdateState.AttemptedVersion, report.AutoUpdateState.AttemptedAt.Format(time.RFC3339))
	}
	if report.AutoUpdateState.LastError != "" {
		fmt.Fprintf(rt.Stdout, "auto update error: %s\n", report.AutoUpdateState.LastError)
	}
	if report.Error != "" {
		fmt.Fprintf(rt.Stdout, "update check error: %s\n", report.Error)
	}
}

func printDoctorToolStatus(rt Runtime) {
	for _, tool := range doctorToolStatusJSON() {
		switch {
		case tool.Status == "ok" && tool.Optional:
			fmt.Fprintf(rt.Stdout, "%s: %s (optional)\n", tool.Name, tool.Path)
		case tool.Status == "ok":
			fmt.Fprintf(rt.Stdout, "%s: %s\n", tool.Name, tool.Path)
		case tool.Optional:
			fmt.Fprintf(rt.Stdout, "%s: missing (%s)\n", tool.Name, tool.Reason)
		default:
			fmt.Fprintf(rt.Stdout, "%s: missing\n", tool.Name)
		}
	}
}

func doctorToolStatusJSON() []doctorToolJSON {
	tools := make([]doctorToolJSON, 0, 3)
	for _, name := range []string{"git", "go"} {
		path, err := exec.LookPath(name)
		if err != nil {
			tools = append(tools, doctorToolJSON{Name: name, Status: "missing"})
			continue
		}
		tools = append(tools, doctorToolJSON{Name: name, Status: "ok", Path: path})
	}
	path, err := exec.LookPath("rg")
	if err != nil {
		tools = append(tools, doctorToolJSON{
			Name:     "rg",
			Status:   "missing",
			Optional: true,
			Reason:   "optional; only needed for `szr rg`",
		})
		return tools
	}
	tools = append(tools, doctorToolJSON{Name: "rg", Status: "ok", Path: path, Optional: true})
	return tools
}

func doctorHistoryDiagnostics(rt Runtime, showHistory bool) (*doctorHistoryJSON, error) {
	if !showHistory {
		return nil, nil
	}
	records, err := rt.History.LoadAll()
	if err != nil {
		return nil, err
	}
	summary := history.Summarize(records, 5)
	return &doctorHistoryJSON{
		Commands:        summary.Commands,
		FallbackRate:    summary.FallbackRate,
		FailureRate:     summary.FailureRate,
		TeeRate:         summary.TeeRate,
		Recommendations: workflowcmd.BuildRecommendations(records, 3),
		Hotspots:        workflowcmd.BuildHotspots(records, 3),
	}, nil
}

func printDoctorHistory(rt Runtime, diagnostics *doctorHistoryJSON) {
	fmt.Fprintln(rt.Stdout, "history diagnostics:")
	fmt.Fprintf(rt.Stdout, "  commands: %d\n", diagnostics.Commands)
	fmt.Fprintf(rt.Stdout, "  fallback rate: %.1f%%\n", diagnostics.FallbackRate)
	fmt.Fprintf(rt.Stdout, "  failure rate: %.1f%%\n", diagnostics.FailureRate)
	fmt.Fprintf(rt.Stdout, "  tee rate: %.1f%%\n", diagnostics.TeeRate)
	if len(diagnostics.Recommendations) > 0 {
		fmt.Fprintln(rt.Stdout, "  recommendations:")
		for _, item := range diagnostics.Recommendations {
			fmt.Fprintf(rt.Stdout, "    - [%s] %s\n", item.Kind, item.Action)
		}
	}
	if len(diagnostics.Hotspots) > 0 {
		fmt.Fprintln(rt.Stdout, "  hotspots:")
		for _, item := range diagnostics.Hotspots {
			fmt.Fprintf(rt.Stdout, "    - %s profile=%s avg=%.1f%% fallback=%.1f%% p95=%dms\n", item.Command, item.Profile, item.AveragePct, item.FallbackRate, item.DurationP95MS)
		}
	}
}

func doctorUpdateJSONFromReport(report DoctorReport) doctorUpdateJSON {
	payload := doctorUpdateJSON{
		Enabled:         report.Enabled,
		AutoUpdate:      report.AutoUpdate,
		InstallMethod:   string(report.Method),
		UpdateAvailable: report.UpdateAvailable,
	}
	if report.Interval > 0 {
		payload.Interval = report.Interval.String()
	}
	if !report.CheckedAt.IsZero() {
		payload.CheckedAt = report.CheckedAt.Format(time.RFC3339)
	}
	payload.LatestVersion = report.LatestVersion
	payload.LatestURL = report.LatestURL
	payload.FromCache = report.FromCache
	payload.UpgradeCommand = report.UpgradeCommand
	if !report.AutoUpdateState.AttemptedAt.IsZero() {
		payload.AttemptedAt = report.AutoUpdateState.AttemptedAt.Format(time.RFC3339)
	}
	payload.AttemptedVersion = report.AutoUpdateState.AttemptedVersion
	if !report.AutoUpdateState.SucceededAt.IsZero() {
		payload.SucceededAt = report.AutoUpdateState.SucceededAt.Format(time.RFC3339)
	}
	payload.SucceededVersion = report.AutoUpdateState.SucceededVersion
	payload.LastError = report.AutoUpdateState.LastError
	payload.Error = report.Error
	return payload
}
