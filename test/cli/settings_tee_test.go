package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/profiles"
	"github.com/devr-tools/szr/test/testutil"
)

func TestSettingsTeeRetentionEntries(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	eng := engine.New(cfg, paths, store, profiles.Builtins(cfg.MaxPreviewLines))
	app := cli.NewWithDependencies("test", cfg, paths, store, eng)

	var code int
	var stdout string
	testutil.WithStdin(t, "19\n8\n20\n50\n21\n64\nq\n", func() {
		code, stdout, _ = testutil.RunApp(t, app, "settings")
	})
	if code != 0 {
		t.Fatalf("settings exited with %d\n%s", code, stdout)
	}
	for _, want := range []string{
		"19. tee max file mb",
		"20. tee max dir files",
		"21. tee max dir mb",
		"saved: tee max file mb 8MB",
		"saved: tee max dir files 50",
		"saved: tee max dir mb 64MB",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in settings output:\n%s", want, stdout)
		}
	}

	var saved config.Config
	if err := json.Unmarshal(testutil.MustReadFile(t, paths.ConfigFile), &saved); err != nil {
		t.Fatalf("decode saved config: %v", err)
	}
	if saved.TeeMaxFileMB != 8 || saved.TeeMaxDirFiles != 50 || saved.TeeMaxDirMB != 64 {
		t.Fatalf("expected saved tee retention values, got %#v", saved)
	}
}
