package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
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
	Method          InstallMethod
	UpgradeCommand  string
	LatestVersion   string
	LatestURL       string
	CheckedAt       time.Time
	FromCache       bool
	UpdateAvailable bool
	Error           string
}

type SelfUpdateResult struct {
	Method         InstallMethod
	UpgradeCommand string
}

type Service struct {
	paths        config.Paths
	now          func() time.Time
	readFile     func(string) ([]byte, error)
	writeFile    func(string, []byte, os.FileMode) error
	mkdirAll     func(string, os.FileMode) error
	executable   func() (string, error)
	evalSymlinks func(string) (string, error)
	lookPath     func(string) (string, error)
	getenv       func(string) string
	userHomeDir  func() (string, error)
	fetchLatest  func(context.Context) (Release, error)
	runCommand   func(context.Context, string, []string, io.Writer, io.Writer) error
}

func New(paths config.Paths) *Service {
	return &Service{
		paths:        paths,
		now:          time.Now,
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		executable:   os.Executable,
		evalSymlinks: filepath.EvalSymlinks,
		lookPath:     exec.LookPath,
		getenv:       os.Getenv,
		userHomeDir:  os.UserHomeDir,
		fetchLatest:  fetchLatestRelease,
		runCommand:   runCommand,
	}
}

func (s *Service) Doctor(ctx context.Context, currentVersion string, cfg config.UpdateCheck) DoctorReport {
	method, upgradeCommand := s.detectInstallMethod()
	report := DoctorReport{
		Enabled:        cfg.Enabled,
		Interval:       time.Duration(cfg.IntervalHours) * time.Hour,
		Method:         method,
		UpgradeCommand: upgradeCommand,
	}
	if !cfg.Enabled {
		return report
	}

	cache, cacheErr := s.loadCache()
	if cacheErr == nil && !cache.CheckedAt.IsZero() && s.now().Sub(cache.CheckedAt) < report.Interval {
		report.LatestVersion = cache.LatestVersion
		report.LatestURL = cache.LatestURL
		report.CheckedAt = cache.CheckedAt
		report.FromCache = true
		report.UpdateAvailable = compareVersions(currentVersion, cache.LatestVersion) < 0
		return report
	}

	release, err := s.fetchLatest(ctx)
	if err != nil {
		if cacheErr == nil && !cache.CheckedAt.IsZero() {
			report.LatestVersion = cache.LatestVersion
			report.LatestURL = cache.LatestURL
			report.CheckedAt = cache.CheckedAt
			report.FromCache = true
			report.UpdateAvailable = compareVersions(currentVersion, cache.LatestVersion) < 0
		}
		report.Error = err.Error()
		return report
	}

	report.LatestVersion = release.Version
	report.LatestURL = release.URL
	report.CheckedAt = s.now().UTC()
	report.UpdateAvailable = compareVersions(currentVersion, release.Version) < 0
	if writeErr := s.saveCache(cachedRelease{
		CheckedAt:     report.CheckedAt,
		LatestVersion: release.Version,
		LatestURL:     release.URL,
	}); writeErr != nil {
		report.Error = writeErr.Error()
	}
	return report
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
		return result, s.runCommand(ctx, "brew", []string{"upgrade", "szr"}, stdout, stderr)
	case InstallMethodGo:
		if _, err := s.lookPath("go"); err != nil {
			return result, fmt.Errorf("go is not installed or not on PATH")
		}
		return result, s.runCommand(ctx, "go", []string{"install", goInstallRef}, stdout, stderr)
	default:
		return result, errors.New("unable to determine how this szr binary was installed")
	}
}

type cachedRelease struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
	LatestURL     string    `json:"latest_url"`
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

func (s *Service) detectInstallMethod() (InstallMethod, string) {
	execPath, err := s.executable()
	if err != nil {
		return InstallMethodUnknown, ""
	}
	resolved := execPath
	if path, err := s.evalSymlinks(execPath); err == nil {
		resolved = path
	}
	resolved = filepath.Clean(resolved)

	cellarFragment := string(filepath.Separator) + "Cellar" + string(filepath.Separator) + "szr" + string(filepath.Separator)
	if strings.Contains(resolved, cellarFragment) {
		return InstallMethodBrew, "brew upgrade szr"
	}

	binName := "szr"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	if gobin := strings.TrimSpace(s.getenv("GOBIN")); gobin != "" && samePath(resolved, filepath.Join(gobin, binName)) {
		return InstallMethodGo, "go install " + goInstallRef
	}

	gopath := strings.TrimSpace(s.getenv("GOPATH"))
	if gopath == "" {
		if homeDir, err := s.userHomeDir(); err == nil {
			gopath = filepath.Join(homeDir, "go")
		}
	}
	if gopath != "" && samePath(resolved, filepath.Join(gopath, "bin", binName)) {
		return InstallMethodGo, "go install " + goInstallRef
	}

	return InstallMethodUnknown, ""
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func compareVersions(current, latest string) int {
	currentParts, currentOK := parseVersion(current)
	latestParts, latestOK := parseVersion(latest)
	if !currentOK || !latestOK {
		return 0
	}
	for i := 0; i < 3; i++ {
		switch {
		case currentParts[i] < latestParts[i]:
			return -1
		case currentParts[i] > latestParts[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(value string) ([3]int, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if value == "" {
		return [3]int{}, false
	}
	parts := strings.SplitN(value, "-", 2)
	fields := strings.Split(parts[0], ".")
	if len(fields) != 3 {
		return [3]int{}, false
	}
	var parsed [3]int
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return [3]int{}, false
		}
		parsed[i] = n
	}
	return parsed, true
}

func fetchLatestRelease(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "szr-update-check")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update check failed: %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, err
	}
	if payload.TagName == "" {
		return Release{}, errors.New("update check returned no release tag")
	}
	return Release{Version: payload.TagName, URL: payload.HTMLURL}, nil
}

func runCommand(ctx context.Context, name string, args []string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
