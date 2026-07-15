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

func brewVerifyService(t *testing.T, root string) *Service {
	t.Helper()
	return &Service{
		paths: testPaths(root),
		now:   func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
		executable: func() (string, error) {
			return filepath.Join(string(filepath.Separator), "opt", "homebrew", "Cellar", "szr", "0.1.0", "bin", "szr"), nil
		},
		evalSymlinks: func(path string) (string, error) { return path, nil },
		lookPath:     func(string) (string, error) { return "ok", nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		fetchLatest: func(context.Context) (Release, error) {
			return Release{Version: "v0.2.0", URL: "https://example.com/v0.2.0"}, nil
		},
	}
}

func brewVerifyConfig() config.UpdateCheck {
	cfg := config.Default().UpdateCheck
	cfg.Enabled = true
	cfg.AutoUpdate = true
	cfg.IntervalHours = 12
	return cfg
}

func TestAutoUpdateNoopUpgradeIsNotRecordedAsSuccess(t *testing.T) {
	root := t.TempDir()
	svc := brewVerifyService(t, root)
	var brewCalls int
	svc.runBrewUpdate = func(context.Context, io.Writer, io.Writer) error { return nil }
	svc.runBrew = func(context.Context, io.Writer, io.Writer) error {
		brewCalls++
		return nil
	}
	svc.installedVersion = func(context.Context) (string, error) { return "v0.1.0", nil }

	cfg := brewVerifyConfig()
	result := svc.AutoUpdate(context.Background(), "v0.1.0", cfg, io.Discard, io.Discard)
	if !result.Attempted || result.Updated || brewCalls != 1 {
		t.Fatalf("unexpected noop result=%#v calls=%d", result, brewCalls)
	}
	if !strings.Contains(result.Error, "still reports v0.1.0") {
		t.Fatalf("expected stale-version error, got %q", result.Error)
	}

	cache, err := svc.loadCache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if !cache.AutoUpdateSucceededAt.IsZero() || cache.AutoUpdateSucceededVersion != "" {
		t.Fatalf("noop upgrade recorded success: %#v", cache)
	}
	if !strings.Contains(cache.AutoUpdateError, "still reports v0.1.0") {
		t.Fatalf("expected cached stale-version error, got %q", cache.AutoUpdateError)
	}

	report := svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.AutoUpdateState.SucceededAt.IsZero() || report.AutoUpdateState.LastError == "" {
		t.Fatalf("doctor should surface the noop honestly: %#v", report.AutoUpdateState)
	}

	result = svc.AutoUpdate(context.Background(), "v0.1.0", cfg, io.Discard, io.Discard)
	if !result.Attempted || result.Updated || brewCalls != 1 || result.Error == "" {
		t.Fatalf("expected cached noop attempt without retry, got %#v calls=%d", result, brewCalls)
	}
}

func TestAutoUpdateVerifiedUpgradeRecordsSuccess(t *testing.T) {
	root := t.TempDir()
	svc := brewVerifyService(t, root)
	svc.runBrewUpdate = func(context.Context, io.Writer, io.Writer) error { return nil }
	svc.runBrew = func(context.Context, io.Writer, io.Writer) error { return nil }
	svc.installedVersion = func(context.Context) (string, error) { return "v0.2.0", nil }

	result := svc.AutoUpdate(context.Background(), "v0.1.0", brewVerifyConfig(), io.Discard, io.Discard)
	if !result.Attempted || !result.Updated || result.Error != "" {
		t.Fatalf("expected verified success, got %#v", result)
	}
	cache, err := svc.loadCache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if cache.AutoUpdateSucceededVersion != "v0.2.0" || cache.AutoUpdateError != "" {
		t.Fatalf("unexpected cache after verified success: %#v", cache)
	}
}

func TestAutoUpdateUnverifiableUpgradeIsNotRecordedAsSuccess(t *testing.T) {
	root := t.TempDir()
	svc := brewVerifyService(t, root)
	svc.runBrewUpdate = func(context.Context, io.Writer, io.Writer) error { return nil }
	svc.runBrew = func(context.Context, io.Writer, io.Writer) error { return nil }
	svc.installedVersion = func(context.Context) (string, error) { return "", errors.New("exec failed") }

	result := svc.AutoUpdate(context.Background(), "v0.1.0", brewVerifyConfig(), io.Discard, io.Discard)
	if !result.Attempted || result.Updated || !strings.Contains(result.Error, "could not be verified") {
		t.Fatalf("expected verification failure, got %#v", result)
	}
	cache, err := svc.loadCache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if !cache.AutoUpdateSucceededAt.IsZero() || cache.AutoUpdateError == "" {
		t.Fatalf("unexpected cache after verification failure: %#v", cache)
	}
}

func TestSelfUpdateBrewRefreshesFormulaeBeforeUpgrade(t *testing.T) {
	root := t.TempDir()
	svc := brewVerifyService(t, root)
	var order []string
	svc.runBrewUpdate = func(context.Context, io.Writer, io.Writer) error {
		order = append(order, "update")
		return nil
	}
	svc.runBrew = func(context.Context, io.Writer, io.Writer) error {
		order = append(order, "upgrade")
		return nil
	}

	result, err := svc.SelfUpdate(context.Background(), io.Discard, io.Discard)
	if err != nil || result.Method != InstallMethodBrew {
		t.Fatalf("self update: result=%#v err=%v", result, err)
	}
	if strings.Join(order, ",") != "update,upgrade" {
		t.Fatalf("expected brew update before upgrade, got %v", order)
	}
}

func TestSelfUpdateBrewUpdateFailureStillUpgrades(t *testing.T) {
	root := t.TempDir()
	svc := brewVerifyService(t, root)
	var upgraded bool
	svc.runBrewUpdate = func(context.Context, io.Writer, io.Writer) error { return errors.New("tap unreachable") }
	svc.runBrew = func(context.Context, io.Writer, io.Writer) error {
		upgraded = true
		return nil
	}

	var stderr strings.Builder
	if _, err := svc.SelfUpdate(context.Background(), io.Discard, &stderr); err != nil {
		t.Fatalf("self update: %v", err)
	}
	if !upgraded {
		t.Fatal("expected upgrade to run despite brew update failure")
	}
	if !strings.Contains(stderr.String(), "brew update failed") {
		t.Fatalf("expected brew update warning, got %q", stderr.String())
	}
}
