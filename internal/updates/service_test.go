package updates

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/config"
)

func TestDoctorUsesLiveFetchAndCache(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	cfg := config.Default().UpdateCheck
	cfg.Enabled = true

	var fetchCalls int
	svc := &Service{
		paths:        paths,
		now:          func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC) },
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		executable:   func() (string, error) { return filepath.Join(root, "bin", "szr"), nil },
		evalSymlinks: func(path string) (string, error) { return path, nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		fetchLatest: func(context.Context) (Release, error) {
			fetchCalls++
			return Release{Version: "v0.2.0", URL: "https://example.com/v0.2.0"}, nil
		},
	}

	report := svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.Enabled || report.LatestVersion != "v0.2.0" || !report.UpdateAvailable || report.FromCache {
		t.Fatalf("unexpected live report: %#v", report)
	}
	if fetchCalls != 1 {
		t.Fatalf("expected one fetch call, got %d", fetchCalls)
	}

	report = svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.FromCache || report.LatestVersion != "v0.2.0" || !report.UpdateAvailable {
		t.Fatalf("unexpected cached report: %#v", report)
	}
	if fetchCalls != 1 {
		t.Fatalf("expected cached read to avoid refetch, got %d", fetchCalls)
	}
}

func TestDoctorFallsBackToCacheOnFetchError(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	svc := &Service{
		paths:        paths,
		now:          func() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) },
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		executable:   func() (string, error) { return filepath.Join(root, "bin", "szr"), nil },
		evalSymlinks: func(path string) (string, error) { return path, nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		fetchLatest:  func(context.Context) (Release, error) { return Release{}, errors.New("network down") },
	}
	if err := svc.saveCache(cachedRelease{
		CheckedAt:     time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC),
		LatestVersion: "v0.2.0",
		LatestURL:     "https://example.com/v0.2.0",
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	cfg := config.Default().UpdateCheck
	cfg.Enabled = true
	report := svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.FromCache || report.LatestVersion != "v0.2.0" || report.Error == "" {
		t.Fatalf("unexpected fallback report: %#v", report)
	}
}

func TestDetectInstallMethod(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)

	cases := []struct {
		name       string
		execPath   string
		gobin      string
		home       string
		wantMethod InstallMethod
		wantCmd    string
	}{
		{
			name:       "brew",
			execPath:   filepath.Join(string(filepath.Separator), "opt", "homebrew", "Cellar", "szr", "0.1.0", "bin", "szr"),
			wantMethod: InstallMethodBrew,
			wantCmd:    "brew upgrade szr",
		},
		{
			name:       "gobin",
			execPath:   filepath.Join(root, "custom-bin", "szr"),
			gobin:      filepath.Join(root, "custom-bin"),
			wantMethod: InstallMethodGo,
			wantCmd:    "go install " + goInstallRef,
		},
		{
			name:       "default gopath",
			execPath:   filepath.Join(root, "go", "bin", "szr"),
			home:       root,
			wantMethod: InstallMethodGo,
			wantCmd:    "go install " + goInstallRef,
		},
		{
			name:       "unknown",
			execPath:   filepath.Join(root, ".local", "bin", "szr"),
			wantMethod: InstallMethodUnknown,
			wantCmd:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{
				paths: paths,
				executable: func() (string, error) {
					return tc.execPath, nil
				},
				evalSymlinks: func(path string) (string, error) { return path, nil },
				getenv: func(key string) string {
					if key == "GOBIN" {
						return tc.gobin
					}
					return ""
				},
				userHomeDir: func() (string, error) { return tc.home, nil },
			}
			method, cmd := svc.detectInstallMethod()
			if method != tc.wantMethod || cmd != tc.wantCmd {
				t.Fatalf("unexpected install detection method=%s cmd=%q", method, cmd)
			}
		})
	}
}

func TestSelfUpdateRunsExpectedCommand(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	var ranName string
	var ranArgs []string
	svc := &Service{
		paths: paths,
		executable: func() (string, error) {
			return filepath.Join(root, "go", "bin", "szr"), nil
		},
		evalSymlinks: func(path string) (string, error) { return path, nil },
		lookPath:     func(string) (string, error) { return "ok", nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		runGoInstall: func(_ context.Context, stdout, stderr io.Writer) error {
			ranName = "go"
			ranArgs = []string{"install", goInstallRef}
			_, _ = io.WriteString(stdout, "updated\n")
			return nil
		},
		runBrew: func(context.Context, io.Writer, io.Writer) error {
			t.Fatal("unexpected brew update call")
			return nil
		},
	}

	var stdout strings.Builder
	result, err := svc.SelfUpdate(context.Background(), &stdout, io.Discard)
	if err != nil {
		t.Fatalf("self update: %v", err)
	}
	if result.Method != InstallMethodGo || ranName != "go" || strings.Join(ranArgs, " ") != "install "+goInstallRef {
		t.Fatalf("unexpected self update result=%#v ran=%s %v", result, ranName, ranArgs)
	}
	if stdout.String() != "updated\n" {
		t.Fatalf("expected forwarded stdout, got %q", stdout.String())
	}
}

func TestAutoUpdateRunsOncePerVersion(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	cfg := config.Default().UpdateCheck
	cfg.Enabled = true
	cfg.AutoUpdate = true
	cfg.IntervalHours = 12

	var updateCalls int
	svc := &Service{
		paths:        paths,
		now:          func() time.Time { return time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC) },
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		executable:   func() (string, error) { return filepath.Join(root, "go", "bin", "szr"), nil },
		evalSymlinks: func(path string) (string, error) { return path, nil },
		lookPath:     func(string) (string, error) { return "ok", nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		fetchLatest: func(context.Context) (Release, error) {
			return Release{Version: "v0.2.0", URL: "https://example.com/v0.2.0"}, nil
		},
		runGoInstall: func(_ context.Context, stdout, stderr io.Writer) error {
			updateCalls++
			_, _ = io.WriteString(stdout, "updated\n")
			return nil
		},
		installedVersion: func(context.Context) (string, error) { return "v0.2.0", nil },
	}

	var stdout strings.Builder
	result := svc.AutoUpdate(context.Background(), "v0.1.0", cfg, &stdout, io.Discard)
	if !result.Attempted || !result.Updated || updateCalls != 1 {
		t.Fatalf("unexpected first auto update result=%#v calls=%d", result, updateCalls)
	}

	result = svc.AutoUpdate(context.Background(), "v0.1.0", cfg, &stdout, io.Discard)
	if !result.Attempted || result.Updated || updateCalls != 1 {
		t.Fatalf("unexpected second auto update result=%#v calls=%d", result, updateCalls)
	}

	report := svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.AutoUpdate || report.AutoUpdateState.SucceededVersion != "v0.2.0" || report.AutoUpdateState.LastError != "" {
		t.Fatalf("unexpected doctor auto update state %#v", report)
	}
}

func TestAutoUpdateFailureIsCachedUntilIntervalExpires(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	cfg := config.Default().UpdateCheck
	cfg.Enabled = true
	cfg.AutoUpdate = true
	cfg.IntervalHours = 12

	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	var updateCalls int
	svc := &Service{
		paths:     paths,
		now:       func() time.Time { return now },
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
		mkdirAll:  os.MkdirAll,
		executable: func() (string, error) {
			return filepath.Join(root, "go", "bin", "szr"), nil
		},
		evalSymlinks: func(path string) (string, error) { return path, nil },
		lookPath:     func(string) (string, error) { return "ok", nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		fetchLatest: func(context.Context) (Release, error) {
			return Release{Version: "v0.2.0", URL: "https://example.com/v0.2.0"}, nil
		},
		runGoInstall: func(_ context.Context, stdout, stderr io.Writer) error {
			updateCalls++
			return errors.New("boom")
		},
	}

	result := svc.AutoUpdate(context.Background(), "v0.1.0", cfg, io.Discard, io.Discard)
	if !result.Attempted || result.Updated || result.Error == "" || updateCalls != 1 {
		t.Fatalf("unexpected first failure result=%#v calls=%d", result, updateCalls)
	}

	result = svc.AutoUpdate(context.Background(), "v0.1.0", cfg, io.Discard, io.Discard)
	if !result.Attempted || result.Updated || updateCalls != 1 {
		t.Fatalf("unexpected cached failure result=%#v calls=%d", result, updateCalls)
	}

	now = now.Add(13 * time.Hour)
	result = svc.AutoUpdate(context.Background(), "v0.1.0", cfg, io.Discard, io.Discard)
	if !result.Attempted || result.Updated || updateCalls != 2 {
		t.Fatalf("unexpected retry after interval result=%#v calls=%d", result, updateCalls)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    int
	}{
		{name: "equal", current: "v1.2.3", latest: "1.2.3", want: 0},
		{name: "update available", current: "v1.2.3", latest: "v1.2.4", want: -1},
		{name: "current newer", current: "1.3.0", latest: "1.2.9", want: 1},
		{name: "prerelease suffix ignored", current: "1.2.3-rc1", latest: "1.2.3", want: 0},
		{name: "invalid current", current: "main", latest: "1.2.3", want: 0},
		{name: "invalid latest", current: "1.2.3", latest: "latest", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareVersions(tc.current, tc.latest); got != tc.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   [3]int
		wantOK bool
	}{
		{name: "plain semver", input: "1.2.3", want: [3]int{1, 2, 3}, wantOK: true},
		{name: "v prefix and suffix", input: "v2.3.4-beta.1", want: [3]int{2, 3, 4}, wantOK: true},
		{name: "blank", input: "", wantOK: false},
		{name: "wrong field count", input: "1.2", wantOK: false},
		{name: "non numeric", input: "1.two.3", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseVersion(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("parseVersion(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("parseVersion(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestAutoUpdateAttemptFresh(t *testing.T) {
	now := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)
	interval := 12 * time.Hour

	cases := []struct {
		name    string
		cache   cachedRelease
		version string
		want    bool
	}{
		{name: "blank version", version: "", want: false},
		{
			name: "already succeeded on version",
			cache: cachedRelease{
				AutoUpdateSucceededVersion: "v0.2.0",
				AutoUpdateSucceededAt:      now.Add(-24 * time.Hour),
			},
			version: "v0.2.0",
			want:    true,
		},
		{
			name: "no matching attempt",
			cache: cachedRelease{
				AutoUpdateAttemptedVersion: "v0.1.9",
				AutoUpdateAttemptedAt:      now.Add(-time.Hour),
			},
			version: "v0.2.0",
			want:    false,
		},
		{
			name: "recent failed attempt still fresh",
			cache: cachedRelease{
				AutoUpdateAttemptedVersion: "v0.2.0",
				AutoUpdateAttemptedAt:      now.Add(-time.Hour),
			},
			version: "v0.2.0",
			want:    true,
		},
		{
			name: "expired failed attempt",
			cache: cachedRelease{
				AutoUpdateAttemptedVersion: "v0.2.0",
				AutoUpdateAttemptedAt:      now.Add(-24 * time.Hour),
			},
			version: "v0.2.0",
			want:    false,
		},
		{
			name: "non positive interval treats attempt as fresh",
			cache: cachedRelease{
				AutoUpdateAttemptedVersion: "v0.2.0",
				AutoUpdateAttemptedAt:      now.Add(-24 * time.Hour),
			},
			version: "v0.2.0",
			want:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			currentInterval := interval
			if tc.name == "non positive interval treats attempt as fresh" {
				currentInterval = 0
			}
			if got := autoUpdateAttemptFresh(tc.cache, tc.version, now, currentInterval); got != tc.want {
				t.Fatalf("autoUpdateAttemptFresh(%+v, %q) = %v, want %v", tc.cache, tc.version, got, tc.want)
			}
		})
	}
}

func testPaths(root string) config.Paths {
	return config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
}
