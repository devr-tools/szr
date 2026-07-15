package updates

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/config"
)

func TestDoctorRefreshBypassesFreshCache(t *testing.T) {
	root := t.TempDir()
	paths := testPaths(root)
	cfg := config.Default().UpdateCheck
	cfg.Enabled = true
	cfg.IntervalHours = 24

	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	latest := "v0.2.0"
	var fetchCalls int
	svc := &Service{
		paths:        paths,
		now:          func() time.Time { return now },
		readFile:     os.ReadFile,
		writeFile:    os.WriteFile,
		mkdirAll:     os.MkdirAll,
		executable:   func() (string, error) { return filepath.Join(root, "bin", "szr"), nil },
		evalSymlinks: func(path string) (string, error) { return path, nil },
		getenv:       func(string) string { return "" },
		userHomeDir:  func() (string, error) { return root, nil },
		fetchLatest: func(context.Context) (Release, error) {
			fetchCalls++
			return Release{Version: latest, URL: "https://example.com/" + latest}, nil
		},
	}
	if err := svc.saveCache(cachedRelease{
		CheckedAt:                  now.Add(-time.Hour),
		LatestVersion:              "v0.1.5",
		LatestURL:                  "https://example.com/v0.1.5",
		AutoUpdateSucceededAt:      now.Add(-2 * time.Hour),
		AutoUpdateSucceededVersion: "v0.1.5",
	}); err != nil {
		t.Fatalf("save cache: %v", err)
	}

	report := svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.FromCache || report.LatestVersion != "v0.1.5" || fetchCalls != 0 {
		t.Fatalf("expected fresh cache to be served, got %#v calls=%d", report, fetchCalls)
	}

	report = svc.DoctorWithOptions(context.Background(), "v0.1.0", cfg, WithRefresh())
	if report.FromCache || report.LatestVersion != latest || !report.UpdateAvailable {
		t.Fatalf("expected live refresh report, got %#v", report)
	}
	if fetchCalls != 1 {
		t.Fatalf("expected refresh to fetch despite fresh cache, got %d calls", fetchCalls)
	}
	if !report.CheckedAt.Equal(now) {
		t.Fatalf("expected refreshed checked_at %s, got %s", now, report.CheckedAt)
	}

	cache, err := svc.loadCache()
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if cache.LatestVersion != latest || !cache.CheckedAt.Equal(now) {
		t.Fatalf("expected refresh to rewrite the cache, got %#v", cache)
	}
	if cache.AutoUpdateSucceededVersion != "v0.1.5" {
		t.Fatalf("expected refresh to preserve auto-update state, got %#v", cache)
	}

	report = svc.Doctor(context.Background(), "v0.1.0", cfg)
	if !report.FromCache || report.LatestVersion != latest || fetchCalls != 1 {
		t.Fatalf("expected rewritten cache to serve later checks, got %#v calls=%d", report, fetchCalls)
	}
}
