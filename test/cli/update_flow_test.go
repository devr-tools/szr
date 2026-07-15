package cli_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/updates"
	"github.com/devr-tools/szr/test/testutil"
)

// orderRecordingUpdater records whether AutoUpdate ran only after the
// dispatched command completed, observed via a marker file the fake command
// writes.
type orderRecordingUpdater struct {
	marker           string
	autoCalls        int
	autoAfterCommand bool
}

func (u *orderRecordingUpdater) DoctorWithOptions(context.Context, string, config.UpdateCheck, ...updates.DoctorOption) updates.DoctorReport {
	return updates.DoctorReport{}
}

func (u *orderRecordingUpdater) AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult {
	u.autoCalls++
	if _, err := os.Stat(u.marker); err == nil {
		u.autoAfterCommand = true
	}
	return updates.AutoUpdateResult{}
}

func (u *orderRecordingUpdater) SelfUpdate(context.Context, io.Writer, io.Writer) (updates.SelfUpdateResult, error) {
	return updates.SelfUpdateResult{}, nil
}

func TestUpdateFlowRunsAfterCommandDispatch(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.AutoUpdate = true
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	marker := filepath.Join(t.TempDir(), "command-ran")
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\ntouch "+marker+"\necho \"## main...origin/main\"\n")

	updater := &orderRecordingUpdater{marker: marker}
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), updater)

	code, _, _ := testutil.RunApp(t, app, "git", "status")
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if updater.autoCalls != 1 {
		t.Fatalf("expected auto update once, got %d", updater.autoCalls)
	}
	if !updater.autoAfterCommand {
		t.Fatal("expected auto update to run after the command completed")
	}
}

// blockingUpdater's Doctor never returns, standing in for a hung update
// probe.
type blockingUpdater struct {
	autoCalls int
}

func (u *blockingUpdater) DoctorWithOptions(ctx context.Context, _ string, _ config.UpdateCheck, _ ...updates.DoctorOption) updates.DoctorReport {
	<-ctx.Done()
	return updates.DoctorReport{}
}

func (u *blockingUpdater) AutoUpdate(context.Context, string, config.UpdateCheck, io.Writer, io.Writer) updates.AutoUpdateResult {
	u.autoCalls++
	return updates.AutoUpdateResult{}
}

func (u *blockingUpdater) SelfUpdate(context.Context, io.Writer, io.Writer) (updates.SelfUpdateResult, error) {
	return updates.SelfUpdateResult{}, nil
}

func TestUpdateFlowDoesNotBlockExitOnHungProbe(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	cfg.UpdateCheck.Enabled = true
	cfg.UpdateCheck.AutoUpdate = true
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "git", "#!/bin/sh\necho \"## main...origin/main\"\n")

	updater := &blockingUpdater{}
	app := cli.NewWithDependenciesAndUpdater("v0.1.0", cfg, paths, nil, testutil.AppEngine(t, paths), updater)

	start := time.Now()
	code, _, _ := testutil.RunApp(t, app, "git", "status")
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("run blocked on hung update probe for %s", elapsed)
	}
	if updater.autoCalls != 0 {
		t.Fatalf("expected auto update to be skipped when the probe never finished, got %d calls", updater.autoCalls)
	}
}

func (u *orderRecordingUpdater) Doctor(ctx context.Context, version string, cfg config.UpdateCheck) updates.DoctorReport {
	return u.DoctorWithOptions(ctx, version, cfg)
}

func (u *blockingUpdater) Doctor(ctx context.Context, version string, cfg config.UpdateCheck) updates.DoctorReport {
	return u.DoctorWithOptions(ctx, version, cfg)
}
