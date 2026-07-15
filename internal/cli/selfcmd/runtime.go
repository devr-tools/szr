package selfcmd

import (
	"context"
	"errors"
	"io"
	"os"

	workflowcmd "github.com/devr-tools/szr/internal/cli/workflows"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/updates"
)

var ErrSelfUpdateUnavailable = errors.New("self update is unavailable in this build")

type Config = config.Config
type Paths = config.Paths
type DoctorReport = updates.DoctorReport
type DoctorOption = updates.DoctorOption

type SelfUpdateResult struct {
	Method         updates.InstallMethod
	UpgradeCommand string
}

type Runtime struct {
	Version      string
	Config       config.Config
	Paths        config.Paths
	History      *history.Store
	Stdout       io.Writer
	Stderr       io.Writer
	SelfUpdate   func(context.Context, *os.File, *os.File) (SelfUpdateResult, error)
	DoctorReport func(context.Context, Config, ...DoctorOption) DoctorReport
}

type doctorArgs struct {
	showHistory bool
	asJSON      bool
	refresh     bool
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
