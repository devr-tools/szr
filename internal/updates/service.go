package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/config"
)

const (
	releaseAPIURL = "https://api.github.com/repos/devr-tools/szr/releases/latest"
	goInstallRef  = "github.com/devr-tools/szr/cmd/szr@latest"
)

type InstallMethod string

const (
	InstallMethodUnknown InstallMethod = "unknown"
	InstallMethodBrew    InstallMethod = "brew"
	InstallMethodGo      InstallMethod = "go-install"
)

type Release struct {
	Version string
	URL     string
}

type DoctorReport struct {
	Enabled         bool
	Interval        time.Duration
	AutoUpdate      bool
	Method          InstallMethod
	UpgradeCommand  string
	LatestVersion   string
	LatestURL       string
	CheckedAt       time.Time
	FromCache       bool
	UpdateAvailable bool
	AutoUpdateState AutoUpdateState
	Error           string
}

type SelfUpdateResult struct {
	Method         InstallMethod
	UpgradeCommand string
}

type AutoUpdateState struct {
	AttemptedAt      time.Time
	AttemptedVersion string
	SucceededAt      time.Time
	SucceededVersion string
	LastError        string
}

type AutoUpdateResult struct {
	Attempted bool
	Updated   bool
	Method    InstallMethod
	Command   string
	Version   string
	Error     string
}

type DoctorOptions struct {
	Refresh bool
}

type DoctorOption func(*DoctorOptions)

func WithRefresh() DoctorOption {
	return func(o *DoctorOptions) { o.Refresh = true }
}

func buildDoctorOptions(opts []DoctorOption) DoctorOptions {
	var options DoctorOptions
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

type Service struct {
	paths            config.Paths
	now              func() time.Time
	readFile         func(string) ([]byte, error)
	writeFile        func(string, []byte, os.FileMode) error
	mkdirAll         func(string, os.FileMode) error
	executable       func() (string, error)
	evalSymlinks     func(string) (string, error)
	lookPath         func(string) (string, error)
	getenv           func(string) string
	userHomeDir      func() (string, error)
	fetchLatest      func(context.Context) (Release, error)
	runBrewUpdate    func(context.Context, io.Writer, io.Writer) error
	runBrew          func(context.Context, io.Writer, io.Writer) error
	runGoInstall     func(context.Context, io.Writer, io.Writer) error
	installedVersion func(context.Context) (string, error)
}

func New(paths config.Paths) *Service {
	s := &Service{
		paths:         paths,
		now:           time.Now,
		readFile:      os.ReadFile,
		writeFile:     os.WriteFile,
		mkdirAll:      os.MkdirAll,
		executable:    os.Executable,
		evalSymlinks:  filepath.EvalSymlinks,
		lookPath:      exec.LookPath,
		getenv:        os.Getenv,
		userHomeDir:   os.UserHomeDir,
		fetchLatest:   fetchLatestRelease,
		runBrewUpdate: runBrewFormulaeUpdate,
		runBrew:       runBrewUpgrade,
		runGoInstall:  runGoInstallLatest,
	}
	s.installedVersion = s.reportedBinaryVersion
	return s
}

func (s *Service) Doctor(ctx context.Context, currentVersion string, cfg config.UpdateCheck) DoctorReport {
	return s.DoctorWithOptions(ctx, currentVersion, cfg)
}

func (s *Service) DoctorWithOptions(ctx context.Context, currentVersion string, cfg config.UpdateCheck, opts ...DoctorOption) DoctorReport {
	options := buildDoctorOptions(opts)
	report := s.baseDoctorReport(cfg)
	cache, cacheErr := s.loadCache()
	if cacheErr == nil {
		report.AutoUpdateState = cache.autoUpdateState()
	}
	if !cfg.Enabled {
		return report
	}

	if !options.Refresh && cacheErr == nil && s.cacheFresh(cache, report.Interval) {
		applyCachedRelease(&report, cache, currentVersion)
		return report
	}

	s.liveDoctorCheck(ctx, &report, cache, currentVersion)
	return report
}

func (s *Service) baseDoctorReport(cfg config.UpdateCheck) DoctorReport {
	method, upgradeCommand := s.detectInstallMethod()
	return DoctorReport{
		Enabled:        cfg.Enabled,
		Interval:       time.Duration(cfg.IntervalHours) * time.Hour,
		AutoUpdate:     cfg.AutoUpdate,
		Method:         method,
		UpgradeCommand: upgradeCommand,
	}
}

func (s *Service) liveDoctorCheck(ctx context.Context, report *DoctorReport, cache cachedRelease, currentVersion string) {
	release, err := s.fetchLatest(ctx)
	if err != nil {
		if !cache.CheckedAt.IsZero() {
			applyCachedRelease(report, cache, currentVersion)
		}
		report.Error = err.Error()
		return
	}
	s.recordLiveRelease(report, cache, release, currentVersion)
}

func (s *Service) cacheFresh(cache cachedRelease, interval time.Duration) bool {
	return !cache.CheckedAt.IsZero() && s.now().Sub(cache.CheckedAt) < interval
}

func applyCachedRelease(report *DoctorReport, cache cachedRelease, currentVersion string) {
	report.LatestVersion = cache.LatestVersion
	report.LatestURL = cache.LatestURL
	report.CheckedAt = cache.CheckedAt
	report.FromCache = true
	report.UpdateAvailable = compareVersions(currentVersion, cache.LatestVersion) < 0
}

func (s *Service) recordLiveRelease(report *DoctorReport, cache cachedRelease, release Release, currentVersion string) {
	report.LatestVersion = release.Version
	report.LatestURL = release.URL
	report.CheckedAt = s.now().UTC()
	report.UpdateAvailable = compareVersions(currentVersion, release.Version) < 0
	cache.CheckedAt = report.CheckedAt
	cache.LatestVersion = release.Version
	cache.LatestURL = release.URL
	if writeErr := s.saveCache(cache); writeErr != nil {
		report.Error = writeErr.Error()
	}
}

func (s *Service) AutoUpdate(ctx context.Context, currentVersion string, cfg config.UpdateCheck, stdout, stderr io.Writer) AutoUpdateResult {
	result := AutoUpdateResult{}
	if !cfg.Enabled || !cfg.AutoUpdate {
		return result
	}

	report := s.Doctor(ctx, currentVersion, cfg)
	result.Method = report.Method
	result.Command = report.UpgradeCommand
	result.Version = report.LatestVersion
	if !report.UpdateAvailable || report.UpgradeCommand == "" {
		return result
	}

	cache, err := s.loadCache()
	if err != nil {
		cache = cachedRelease{
			CheckedAt:     report.CheckedAt,
			LatestVersion: report.LatestVersion,
			LatestURL:     report.LatestURL,
		}
	}
	if autoUpdateAttemptFresh(cache, report.LatestVersion, s.now(), report.Interval) {
		result.Attempted = true
		result.Error = cache.AutoUpdateError
		return result
	}

	result.Attempted = true
	cache.CheckedAt = report.CheckedAt
	cache.LatestVersion = report.LatestVersion
	cache.LatestURL = report.LatestURL
	cache.AutoUpdateAttemptedAt = s.now().UTC()
	cache.AutoUpdateAttemptedVersion = report.LatestVersion

	updateResult, updateErr := s.SelfUpdate(ctx, stdout, stderr)
	result.Method = updateResult.Method
	if updateResult.UpgradeCommand != "" {
		result.Command = updateResult.UpgradeCommand
	}
	if updateErr != nil {
		result.Error = updateErr.Error()
		cache.AutoUpdateError = result.Error
		_ = s.saveCache(cache)
		return result
	}
	if verifyErr := s.verifyInstalledVersion(ctx, report.LatestVersion); verifyErr != nil {
		result.Error = verifyErr.Error()
		cache.AutoUpdateError = result.Error
		_ = s.saveCache(cache)
		return result
	}

	result.Updated = true
	cache.AutoUpdateSucceededAt = s.now().UTC()
	cache.AutoUpdateSucceededVersion = report.LatestVersion
	cache.AutoUpdateError = ""
	_ = s.saveCache(cache)
	return result
}

func (s *Service) SelfUpdate(ctx context.Context, stdout, stderr io.Writer) (SelfUpdateResult, error) {
	method, upgradeCommand := s.detectInstallMethod()
	result := SelfUpdateResult{
		Method:         method,
		UpgradeCommand: upgradeCommand,
	}

	switch method {
	case InstallMethodBrew:
		if _, err := s.lookPath("brew"); err != nil {
			return result, fmt.Errorf("brew is not installed or not on PATH")
		}
		if err := s.runBrewUpdate(ctx, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "szr: brew update failed, continuing with upgrade: %v\n", err)
		}
		if err := s.runBrew(ctx, stdout, stderr); err != nil {
			return result, fmt.Errorf("failed to update via %s: %w", result.UpgradeCommand, err)
		}
		return result, nil
	case InstallMethodGo:
		if _, err := s.lookPath("go"); err != nil {
			return result, fmt.Errorf("go is not installed or not on PATH")
		}
		if err := s.runGoInstall(ctx, stdout, stderr); err != nil {
			return result, fmt.Errorf("failed to update via %s: %w", result.UpgradeCommand, err)
		}
		return result, nil
	default:
		return result, errors.New("unable to determine how this szr binary was installed")
	}
}

type cachedRelease struct {
	CheckedAt                  time.Time `json:"checked_at"`
	LatestVersion              string    `json:"latest_version"`
	LatestURL                  string    `json:"latest_url"`
	AutoUpdateAttemptedAt      time.Time `json:"auto_update_attempted_at,omitempty"`
	AutoUpdateAttemptedVersion string    `json:"auto_update_attempted_version,omitempty"`
	AutoUpdateSucceededAt      time.Time `json:"auto_update_succeeded_at,omitempty"`
	AutoUpdateSucceededVersion string    `json:"auto_update_succeeded_version,omitempty"`
	AutoUpdateError            string    `json:"auto_update_error,omitempty"`
}

func (c cachedRelease) autoUpdateState() AutoUpdateState {
	return AutoUpdateState{
		AttemptedAt:      c.AutoUpdateAttemptedAt,
		AttemptedVersion: c.AutoUpdateAttemptedVersion,
		SucceededAt:      c.AutoUpdateSucceededAt,
		SucceededVersion: c.AutoUpdateSucceededVersion,
		LastError:        c.AutoUpdateError,
	}
}

func (s *Service) loadCache() (cachedRelease, error) {
	data, err := s.readFile(s.cachePath())
	if err != nil {
		return cachedRelease{}, err
	}
	var cache cachedRelease
	if err := json.Unmarshal(data, &cache); err != nil {
		return cachedRelease{}, err
	}
	return cache, nil
}

func (s *Service) saveCache(cache cachedRelease) error {
	if err := s.mkdirAll(filepath.Dir(s.cachePath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return s.writeFile(s.cachePath(), append(data, '\n'), 0o644)
}

func (s *Service) cachePath() string {
	return filepath.Join(s.paths.DataDir, "update-check.json")
}

func autoUpdateAttemptFresh(cache cachedRelease, version string, now time.Time, interval time.Duration) bool {
	if strings.TrimSpace(version) == "" {
		return false
	}
	if cache.AutoUpdateSucceededVersion == version && !cache.AutoUpdateSucceededAt.IsZero() {
		return true
	}
	if cache.AutoUpdateAttemptedVersion != version || cache.AutoUpdateAttemptedAt.IsZero() {
		return false
	}
	if interval <= 0 {
		return true
	}
	return now.Sub(cache.AutoUpdateAttemptedAt) < interval
}
