package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	workflowcmd "github.com/devr-tools/szr/internal/cli/workflows"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/selfinstall"
	"github.com/devr-tools/szr/internal/updates"
)

type doctorArgs struct {
	showHistory bool
	asJSON      bool
}

type doctorJSON struct {
	Version             string             `json:"version"`
	Executable          string             `json:"executable,omitempty"`
	InstallDir          string             `json:"install_dir,omitempty"`
	InstallTarget       string             `json:"install_target,omitempty"`
	PathPresent         bool               `json:"path_present"`
	ShellRC             string             `json:"shell_rc,omitempty"`
	PathFix             string             `json:"path_fix,omitempty"`
	Config              string             `json:"config"`
	ConfigDir           string             `json:"config_dir"`
	DataDir             string             `json:"data_dir"`
	History             string             `json:"history"`
	TeeDir              string             `json:"tee_dir"`
	ReasoningBudgetMode string             `json:"reasoning_budget_mode"`
	Update              doctorUpdateJSON   `json:"update"`
	Tools               []doctorToolJSON   `json:"tools"`
	HistoryDiagnostics  *doctorHistoryJSON `json:"history_diagnostics,omitempty"`
	ProjectRules        string             `json:"project_rules,omitempty"`
}

type doctorUpdateJSON struct {
	Enabled          bool   `json:"enabled"`
	Interval         string `json:"interval,omitempty"`
	AutoUpdate       bool   `json:"auto_update"`
	InstallMethod    string `json:"install_method"`
	LatestVersion    string `json:"latest_version,omitempty"`
	LatestURL        string `json:"latest_url,omitempty"`
	CheckedAt        string `json:"checked_at,omitempty"`
	FromCache        bool   `json:"from_cache,omitempty"`
	UpdateAvailable  bool   `json:"update_available"`
	UpgradeCommand   string `json:"upgrade_command,omitempty"`
	AttemptedAt      string `json:"attempted_at,omitempty"`
	AttemptedVersion string `json:"attempted_version,omitempty"`
	SucceededAt      string `json:"succeeded_at,omitempty"`
	SucceededVersion string `json:"succeeded_version,omitempty"`
	LastError        string `json:"last_error,omitempty"`
	Error            string `json:"error,omitempty"`
}

type doctorToolJSON struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Optional bool   `json:"optional,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type doctorHistoryJSON struct {
	Commands        int                          `json:"commands"`
	FallbackRate    float64                      `json:"fallback_rate"`
	FailureRate     float64                      `json:"failure_rate"`
	TeeRate         float64                      `json:"tee_rate"`
	Recommendations []workflowcmd.Recommendation `json:"recommendations,omitempty"`
	Hotspots        []workflowcmd.HotspotStat    `json:"hotspots,omitempty"`
}

func (a *App) runSelf(ctx context.Context, flags globalFlags, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "szr: self requires a subcommand")
		return 2
	}

	switch args[0] {
	case "install":
		return a.runSelfInstall(args[1:])
	case "uninstall":
		return a.runSelfUninstall(args[1:])
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

func (a *App) runSelfUninstall(args []string) int {
	printOnly, overrideDir, code := parseSelfUninstallArgs(args)
	if code != 0 {
		return code
	}

	plan, err := resolveSelfInstallPlan(overrideDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: %v\n", err)
		return 1
	}

	if printOnly {
		printSelfUninstallPlan(plan)
		return 0
	}

	result, err := selfinstall.Uninstall(plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to uninstall szr: %v\n", err)
		return 1
	}

	if result.Removed {
		fmt.Printf("uninstalled: %s\n", result.Plan.TargetPath)
	} else {
		fmt.Printf("uninstalled: already absent (%s)\n", result.Plan.TargetPath)
	}
	if result.ShellConfigured {
		fmt.Printf("shell rc still references install dir: %s\n", result.Plan.ShellRCPath)
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

func parseSelfUninstallArgs(args []string) (bool, string, int) {
	printOnly := false
	overrideDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--print":
			printOnly = true
		case "--path":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "szr: self uninstall requires a directory after --path")
				return false, "", 2
			}
			i++
			overrideDir = args[i]
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown self uninstall flag %s\n", args[i])
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

func printSelfUninstallPlan(plan selfinstall.Plan) {
	fmt.Println("plan: self uninstall")
	fmt.Printf("  target: %s\n", plan.TargetPath)
	if plan.ShellRCPath != "" {
		fmt.Printf("  shell rc: %s\n", plan.ShellRCPath)
	}
	if plan.ShellSnippet != "" {
		fmt.Printf("  path snippet: %s\n", plan.ShellSnippet)
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
	parsed, code := parseDoctorArgs(args)
	if code != 0 {
		return code
	}

	executablePath, plan, planErr := doctorInstallPlan()
	update := a.doctorUpdateReport(ctx, cfg)
	historyDiagnostics, err := a.doctorHistoryDiagnostics(parsed.showHistory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "szr: failed to read history: %v\n", err)
		return 1
	}

	if parsed.asJSON {
		payload := doctorJSON{
			Version:             a.version,
			Executable:          executablePath,
			Config:              a.paths.ConfigFile,
			ConfigDir:           a.paths.ConfigDir,
			DataDir:             a.paths.DataDir,
			History:             a.paths.HistoryFile,
			TeeDir:              a.paths.TeeDir,
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
		if a.paths.ProjectRuleFile != "" {
			payload.ProjectRules = a.paths.ProjectRuleFile
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
		return 0
	}

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
	printDoctorUpdateStatus(update)
	if a.paths.ProjectRuleFile != "" {
		fmt.Printf("project rules: %s\n", a.paths.ProjectRuleFile)
	}
	printDoctorToolStatus()
	if parsed.showHistory && historyDiagnostics != nil {
		printDoctorHistory(historyDiagnostics)
	}
	return 0
}

func parseDoctorArgs(args []string) (doctorArgs, int) {
	parsed := doctorArgs{}
	for _, arg := range args {
		switch arg {
		case "--history":
			parsed.showHistory = true
		case "--json":
			parsed.asJSON = true
		default:
			fmt.Fprintf(os.Stderr, "szr: unknown doctor flag %s\n", arg)
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

func (a *App) doctorUpdateReport(ctx context.Context, cfg config.Config) updates.DoctorReport {
	if a.updater == nil {
		return updates.DoctorReport{}
	}
	return a.updater.Doctor(ctx, a.version, cfg.UpdateCheck)
}

func printDoctorUpdateStatus(report updates.DoctorReport) {
	if !report.Enabled {
		fmt.Println("update checks: disabled")
		fmt.Printf("install method: %s\n", report.Method)
		if report.AutoUpdate {
			fmt.Println("auto update: enabled")
		} else {
			fmt.Println("auto update: disabled")
		}
		return
	}
	fmt.Println("update checks: enabled")
	fmt.Printf("update check interval: %s\n", report.Interval)
	if report.AutoUpdate {
		fmt.Println("auto update: enabled")
	} else {
		fmt.Println("auto update: disabled")
	}
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
	if !report.AutoUpdateState.SucceededAt.IsZero() {
		fmt.Printf("last auto update: %s %s\n", report.AutoUpdateState.SucceededVersion, report.AutoUpdateState.SucceededAt.Format(time.RFC3339))
	} else if !report.AutoUpdateState.AttemptedAt.IsZero() {
		fmt.Printf("last auto update attempt: %s %s\n", report.AutoUpdateState.AttemptedVersion, report.AutoUpdateState.AttemptedAt.Format(time.RFC3339))
	}
	if report.AutoUpdateState.LastError != "" {
		fmt.Printf("auto update error: %s\n", report.AutoUpdateState.LastError)
	}
	if report.Error != "" {
		fmt.Printf("update check error: %s\n", report.Error)
	}
}

func printDoctorToolStatus() {
	for _, tool := range doctorToolStatusJSON() {
		switch {
		case tool.Status == "ok" && tool.Optional:
			fmt.Printf("%s: %s (optional)\n", tool.Name, tool.Path)
		case tool.Status == "ok":
			fmt.Printf("%s: %s\n", tool.Name, tool.Path)
		case tool.Optional:
			fmt.Printf("%s: missing (%s)\n", tool.Name, tool.Reason)
		default:
			fmt.Printf("%s: missing\n", tool.Name)
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

func (a *App) doctorHistoryDiagnostics(showHistory bool) (*doctorHistoryJSON, error) {
	if !showHistory {
		return nil, nil
	}
	records, err := a.history.LoadAll()
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

func printDoctorHistory(diagnostics *doctorHistoryJSON) {
	fmt.Println("history diagnostics:")
	fmt.Printf("  commands: %d\n", diagnostics.Commands)
	fmt.Printf("  fallback rate: %.1f%%\n", diagnostics.FallbackRate)
	fmt.Printf("  failure rate: %.1f%%\n", diagnostics.FailureRate)
	fmt.Printf("  tee rate: %.1f%%\n", diagnostics.TeeRate)
	if len(diagnostics.Recommendations) > 0 {
		fmt.Println("  recommendations:")
		for _, item := range diagnostics.Recommendations {
			fmt.Printf("    - [%s] %s\n", item.Kind, item.Action)
		}
	}
	if len(diagnostics.Hotspots) > 0 {
		fmt.Println("  hotspots:")
		for _, item := range diagnostics.Hotspots {
			fmt.Printf("    - %s profile=%s avg=%.1f%% fallback=%.1f%% p95=%dms\n", item.Command, item.Profile, item.AveragePct, item.FallbackRate, item.DurationP95MS)
		}
	}
}

func doctorUpdateJSONFromReport(report updates.DoctorReport) doctorUpdateJSON {
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
