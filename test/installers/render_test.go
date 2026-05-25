package installers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/installers"
	"github.com/devr-tools/szr/test/testutil"
)

func TestRenderTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module github.com/devr-tools/szr\n")
	testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")

	plans, err := installers.RenderAll(installers.Options{
		RepoRoot: root,
		Binary:   "./dev/szr",
	})
	if err != nil {
		t.Fatalf("render all: %v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("unexpected plan count: %d", len(plans))
	}

	for _, target := range installers.Targets() {
		plan, err := installers.Render(target, installers.Options{
			RepoRoot: root,
			Binary:   "./dev/szr",
		})
		if err != nil {
			t.Fatalf("render %s: %v", target, err)
		}
		if plan.Target != target || plan.Paths.Binary != "./dev/szr" {
			t.Fatalf("unexpected plan metadata: %#v", plan)
		}
		if len(plan.Files) != 3 {
			t.Fatalf("unexpected file count for %s: %d", target, len(plan.Files))
		}
		if len(plan.ManualSteps) != 2 {
			t.Fatalf("unexpected manual steps for %s: %v", target, plan.ManualSteps)
		}

		var sawHook, sawInstallDoc, sawInstruction bool
		for _, file := range plan.Files {
			switch {
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")):
				sawHook = true
				if file.Strategy != installers.StrategyWrite || file.Mode != 0o755 {
					t.Fatalf("unexpected hook file metadata: %#v", file)
				}
				if !strings.Contains(file.Content, "./dev/szr") || !strings.Contains(file.Content, "szr hint") {
					t.Fatalf("unexpected hook content: %q", file.Content)
				}
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "install", string(target)+".md")):
				sawInstallDoc = true
				if !strings.Contains(file.Content, "Hook command:") || !strings.Contains(file.Content, "./.szr/hooks/pre-command.sh") {
					t.Fatalf("unexpected install doc: %q", file.Content)
				}
			default:
				sawInstruction = true
				if target == installers.TargetCursor {
					if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "alwaysApply: true") {
						t.Fatalf("unexpected cursor file: %#v", file)
					}
				} else if target == installers.TargetShell {
					if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "szr_explain()") || !strings.Contains(file.Content, "szr_proxy()") {
						t.Fatalf("unexpected shell file: %#v", file)
					}
				} else {
					if file.Strategy != installers.StrategyMerge || file.Marker == "" {
						t.Fatalf("unexpected merge file: %#v", file)
					}
					if !strings.Contains(file.Content, "explain <cmd...>") || !strings.Contains(file.Content, "proxy <cmd...>") {
						t.Fatalf("unexpected instruction body: %q", file.Content)
					}
				}
			}
		}
		if !sawHook || !sawInstallDoc || !sawInstruction {
			t.Fatalf("missing generated files for %s", target)
		}
	}

	if _, err := installers.Render("unknown", installers.Options{RepoRoot: root}); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestRenderUninstallTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module github.com/devr-tools/szr\n")
	testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")

	plans, err := installers.RenderAllUninstall(installers.Options{
		RepoRoot: root,
		Binary:   "./dev/szr",
	})
	if err != nil {
		t.Fatalf("render all uninstall: %v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("unexpected uninstall plan count: %d", len(plans))
	}

	for _, target := range installers.Targets() {
		plan, err := installers.RenderUninstall(target, installers.Options{
			RepoRoot: root,
			Binary:   "./dev/szr",
		})
		if err != nil {
			t.Fatalf("render uninstall %s: %v", target, err)
		}
		if plan.Target != target || plan.Paths.Binary != "./dev/szr" {
			t.Fatalf("unexpected uninstall plan metadata: %#v", plan)
		}
		if len(plan.ManualSteps) != 1 {
			t.Fatalf("unexpected uninstall manual steps for %s: %v", target, plan.ManualSteps)
		}
		if len(plan.Files) < 2 {
			t.Fatalf("unexpected uninstall file count for %s: %d", target, len(plan.Files))
		}
		if target == installers.TargetShell {
			if !strings.Contains(plan.ManualSteps[0], ".szr/install/shell.sh") {
				t.Fatalf("unexpected shell uninstall manual step: %q", plan.ManualSteps[0])
			}
		} else if !strings.Contains(plan.ManualSteps[0], ".szr/hooks/pre-command.sh") {
			t.Fatalf("unexpected uninstall manual step: %q", plan.ManualSteps[0])
		}

		var sawInstallDoc, sawTargetFile bool
		for _, file := range plan.Files {
			switch {
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "install", string(target)+".md")):
				sawInstallDoc = true
				if file.Strategy != installers.StrategyDelete {
					t.Fatalf("unexpected uninstall doc metadata: %#v", file)
				}
			case strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")):
				if file.Strategy != installers.StrategyDelete {
					t.Fatalf("unexpected uninstall hook metadata: %#v", file)
				}
			default:
				sawTargetFile = true
				switch target {
				case installers.TargetCursor, installers.TargetShell:
					if file.Strategy != installers.StrategyDelete {
						t.Fatalf("unexpected uninstall delete file: %#v", file)
					}
				default:
					if file.Strategy != installers.StrategyUnmerge || file.Marker == "" {
						t.Fatalf("unexpected uninstall merge removal file: %#v", file)
					}
				}
			}
		}
		if !sawInstallDoc || !sawTargetFile {
			t.Fatalf("missing uninstall files for %s", target)
		}
	}

	if _, err := installers.RenderUninstall("unknown", installers.Options{RepoRoot: root}); err == nil {
		t.Fatal("expected unknown uninstall target error")
	}
}

func TestRenderAndDetectEdgeErrors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	if _, err := installers.DetectPathsWith(filepath.Join(root, "missing"), os.Stat); err == nil {
		t.Fatal("expected detect paths missing-root error")
	}

	if _, err := installers.RenderAll(installers.Options{RepoRoot: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("expected render all error for missing repo root")
	}
}
