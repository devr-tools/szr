package installers_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/installers"
	"github.com/devr-tools/szr/test/testutil"
)

func TestApplyPlanMergeAndIdempotence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Existing\n")

	plan, err := installers.Render(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
	})
	if err != nil {
		t.Fatalf("render codex: %v", err)
	}

	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply first pass: %v", err)
	}
	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply second pass: %v", err)
	}

	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read agents: %v", err)
	}
	content := string(agents)
	if !strings.Contains(content, "# Existing") {
		t.Fatalf("existing content lost: %q", content)
	}
	if strings.Count(content, "<!-- szr-codex:begin -->") != 1 {
		t.Fatalf("expected single codex block: %q", content)
	}
	if !strings.Contains(content, "## szr for Codex") || !strings.Contains(content, "./bin/szr proxy <cmd...>") {
		t.Fatalf("unexpected agents content: %q", content)
	}

	hookInfo, err := os.Stat(filepath.Join(root, ".szr", "hooks", "pre-command.sh"))
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	if runtime.GOOS != "windows" && hookInfo.Mode()&0o111 == 0 {
		t.Fatalf("hook not executable: %v", hookInfo.Mode())
	}

	doc, err := os.ReadFile(filepath.Join(root, ".szr", "install", "codex.md"))
	if err != nil {
		t.Fatalf("read install doc: %v", err)
	}
	if !strings.Contains(string(doc), "Instruction file: ./AGENTS.md") {
		t.Fatalf("unexpected install doc: %q", string(doc))
	}
}

func TestApplyWithErrors(t *testing.T) {
	t.Parallel()

	plan := installers.Plan{
		Files: []installers.File{{
			Path:     "/tmp/file.txt",
			Content:  "x",
			Mode:     0o644,
			Strategy: installers.StrategyWrite,
		}},
	}

	wantErr := errors.New("boom")
	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return wantErr },
		func(string) ([]byte, error) { return nil, nil },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected mkdir error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, wantErr },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected read error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return wantErr },
		func(string, os.FileMode) error { return nil },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected write error, got %v", err)
	}

	if err := installers.ApplyWith(
		plan,
		func(string, os.FileMode) error { return nil },
		func(string) ([]byte, error) { return nil, os.ErrNotExist },
		func(string, []byte, os.FileMode) error { return nil },
		func(string, os.FileMode) error { return wantErr },
		func(string) error { return nil },
	); !errors.Is(err, wantErr) {
		t.Fatalf("expected chmod error, got %v", err)
	}
}

func TestApplyEmptyMergePlan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	plan := installers.Plan{
		Files: []installers.File{{
			Path:     filepath.Join(root, "AGENTS.md"),
			Content:  "",
			Mode:     0o644,
			Strategy: installers.StrategyMerge,
			Marker:   "szr-empty",
		}},
	}
	if err := installers.Apply(plan); err != nil {
		t.Fatalf("apply empty merge plan: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read empty merge file: %v", err)
	}
	if !strings.Contains(string(data), "<!-- szr-empty:begin -->") {
		t.Fatalf("expected empty merge markers, got %q", string(data))
	}
}

func TestApplyUninstallPlanRemovesManagedContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "AGENTS.md"), "# Existing\n")

	installPlan, err := installers.Render(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
	})
	if err != nil {
		t.Fatalf("render install plan: %v", err)
	}
	if err := installers.Apply(installPlan); err != nil {
		t.Fatalf("apply install plan: %v", err)
	}

	uninstallPlan, err := installers.RenderUninstall(installers.TargetCodex, installers.Options{
		RepoRoot: root,
		Binary:   "./bin/szr",
	})
	if err != nil {
		t.Fatalf("render uninstall plan: %v", err)
	}
	if err := installers.Apply(uninstallPlan); err != nil {
		t.Fatalf("apply uninstall plan: %v", err)
	}

	agents := string(testutil.MustReadFile(t, filepath.Join(root, "AGENTS.md")))
	if agents != "# Existing\n" {
		t.Fatalf("expected original AGENTS content to survive, got %q", agents)
	}
	for _, path := range []string{
		filepath.Join(root, ".szr", "hooks", "pre-command.sh"),
		filepath.Join(root, ".szr", "install", "codex.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, got err=%v", path, err)
		}
	}
}
