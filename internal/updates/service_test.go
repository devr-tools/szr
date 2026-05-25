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
		runCommand: func(_ context.Context, name string, args []string, stdout, stderr io.Writer) error {
			ranName = name
			ranArgs = append([]string(nil), args...)
			_, _ = io.WriteString(stdout, "updated\n")
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

func testPaths(root string) config.Paths {
	return config.Paths{
		ConfigDir:   filepath.Join(root, "config"),
		ConfigFile:  filepath.Join(root, "config", "config.json"),
		DataDir:     filepath.Join(root, "data"),
		HistoryFile: filepath.Join(root, "data", "history.jsonl"),
		TeeDir:      filepath.Join(root, "data", "tee"),
	}
}
