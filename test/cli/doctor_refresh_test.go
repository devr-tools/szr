package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/updates"
	"github.com/devr-tools/szr/test/testutil"
)

// refreshRecordingUpdater captures the refresh option passed to each Doctor
// call so tests can assert flag plumbing from the CLI layer.
type refreshRecordingUpdater struct {
	report       updates.DoctorReport
	refreshCalls []bool
}

func (u *refreshRecordingUpdater) DoctorWithOptions(_ context.Context, _ string, _ config.UpdateCheck, opts ...updates.DoctorOption) updates.DoctorReport {
	var options updates.DoctorOptions
	for _, opt := range opts {
		opt(&options)
	}
	u.refreshCalls = append(u.refreshCalls, options.Refresh)
	return u.report
}

func (u *refreshRecordingUpdater) AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult {
	return updates.AutoUpdateResult{}
}

func (u *refreshRecordingUpdater) SelfUpdate(context.Context, io.Writer, io.Writer) (updates.SelfUpdateResult, error) {
	return updates.SelfUpdateResult{}, nil
}

func refreshDoctorApp(t *testing.T) (*cli.App, *refreshRecordingUpdater) {
	t.Helper()
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	updater := &refreshRecordingUpdater{
		report: updates.DoctorReport{
			Enabled:       true,
			Interval:      24 * time.Hour,
			Method:        updates.InstallMethodBrew,
			LatestVersion: "v0.2.0",
			LatestURL:     "https://example.com/v0.2.0",
			CheckedAt:     time.Date(2026, 7, 15, 20, 30, 0, 0, time.UTC),
		},
	}
	return cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), updater), updater
}

func TestSelfDoctorRefreshFlagRequestsLiveCheck(t *testing.T) {
	app, updater := refreshDoctorApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor", "--refresh")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if len(updater.refreshCalls) != 1 || !updater.refreshCalls[0] {
		t.Fatalf("expected one refreshed doctor call, got %v", updater.refreshCalls)
	}
	if !strings.Contains(stdout, "latest stable: v0.2.0") {
		t.Fatalf("expected live latest version in output, got %q", stdout)
	}
}

func TestSelfDoctorWithoutRefreshFlagUsesDefaultCheck(t *testing.T) {
	app, updater := refreshDoctorApp(t)

	code, _, stderr := testutil.RunApp(t, app, "self", "doctor")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor stderr=%q code=%d", stderr, code)
	}
	if len(updater.refreshCalls) != 1 || updater.refreshCalls[0] {
		t.Fatalf("expected one non-refresh doctor call, got %v", updater.refreshCalls)
	}
}

func TestSelfDoctorRefreshCombinesWithJSON(t *testing.T) {
	app, updater := refreshDoctorApp(t)

	code, stdout, stderr := testutil.RunApp(t, app, "self", "doctor", "--json", "--refresh")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected self doctor json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if len(updater.refreshCalls) != 1 || !updater.refreshCalls[0] {
		t.Fatalf("expected one refreshed doctor call, got %v", updater.refreshCalls)
	}

	var payload struct {
		Update struct {
			LatestVersion string `json:"latest_version"`
			CheckedAt     string `json:"checked_at"`
			FromCache     bool   `json:"from_cache"`
		} `json:"update"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode self doctor json: %v", err)
	}
	if payload.Update.FromCache || payload.Update.LatestVersion != "v0.2.0" || payload.Update.CheckedAt == "" {
		t.Fatalf("expected live update payload, got %#v", payload.Update)
	}
}

func TestDoctorAliasAcceptsRefreshFlag(t *testing.T) {
	app, updater := refreshDoctorApp(t)

	code, _, stderr := testutil.RunApp(t, app, "doctor", "--refresh")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected doctor stderr=%q code=%d", stderr, code)
	}
	if len(updater.refreshCalls) != 1 || !updater.refreshCalls[0] {
		t.Fatalf("expected one refreshed doctor call, got %v", updater.refreshCalls)
	}
}

func (u *refreshRecordingUpdater) Doctor(ctx context.Context, version string, cfg config.UpdateCheck) updates.DoctorReport {
	return u.DoctorWithOptions(ctx, version, cfg)
}
