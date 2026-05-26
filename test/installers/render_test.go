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
	home := t.TempDir()

	plans, err := installers.RenderAll(installers.Options{
		RepoRoot: root,
		Binary:   "./dev/szr",
		HomeDir:  home,
	})
	if err != nil {
		t.Fatalf("render all: %v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("unexpected plan count: %d", len(plans))
	}

	for _, target := range installers.Targets() {
		t.Run(string(target), func(t *testing.T) {
			plan, err := installers.Render(target, installers.Options{
				RepoRoot: root,
				Binary:   "./dev/szr",
				HomeDir:  home,
			})
			if err != nil {
				t.Fatalf("render %s: %v", target, err)
			}
			assertInstallRenderPlan(t, target, plan)
		})
	}

	if _, err := installers.Render("unknown", installers.Options{RepoRoot: root}); err == nil {
		t.Fatal("expected unknown target error")
	}
}

func TestRenderClaudeTarget(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	plan, err := installers.Render(installers.TargetClaude, installers.Options{
		HomeDir: home,
		Binary:  "szr",
	})
	if err != nil {
		t.Fatalf("render claude: %v", err)
	}
	if !plan.Paths.Global {
		t.Fatalf("expected global plan metadata: %#v", plan.Paths)
	}
	if len(plan.Files) != 5 {
		t.Fatalf("unexpected claude file count: %d", len(plan.Files))
	}

	var sawHook, sawInstallDoc, sawSZRDoc, sawClaudeMD, sawSettings bool
	for _, file := range plan.Files {
		switch {
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")):
			sawHook = true
			if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--hook claude") {
				t.Fatalf("unexpected global hook file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", ".szr", "install", "claude-code.md")):
			sawInstallDoc = true
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")):
			sawSZRDoc = true
			if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Claude Code") {
				t.Fatalf("unexpected global szr.md file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")):
			sawClaudeMD = true
			if file.Strategy != installers.StrategyMerge || file.Marker != "szr-claude-code-global" {
				t.Fatalf("unexpected global CLAUDE.md file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")):
			sawSettings = true
			if file.Strategy != installers.StrategyClaudeSettingsMerge {
				t.Fatalf("unexpected global settings file: %#v", file)
			}
		}
	}
	if !sawHook || !sawInstallDoc || !sawSZRDoc || !sawClaudeMD || !sawSettings {
		t.Fatalf("missing generated files for claude plan: %#v", plan.Files)
	}
}

func assertInstallRenderPlan(t *testing.T, target installers.Target, plan installers.Plan) {
	t.Helper()
	if plan.Target != target || plan.Paths.Binary != "./dev/szr" {
		t.Fatalf("unexpected plan metadata: %#v", plan)
	}
	expectedFiles := 3
	if target == installers.TargetClaude {
		expectedFiles = 5
	}
	if len(plan.Files) != expectedFiles {
		t.Fatalf("unexpected file count for %s: %d", target, len(plan.Files))
	}
	if len(plan.ManualSteps) != 2 {
		t.Fatalf("unexpected manual steps for %s: %v", target, plan.ManualSteps)
	}

	var sawHook, sawInstallDoc, sawInstruction bool
	for _, file := range plan.Files {
		switch {
		case strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")):
			sawHook = true
			assertInstallHookFile(t, file)
		case strings.HasSuffix(file.Path, filepath.Join(".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".claude", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".codex", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".cursor", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".gemini", ".szr", "install", string(target)+".md")):
			sawInstallDoc = true
			assertInstallDocFile(t, file, target)
		default:
			sawInstruction = true
			assertInstallInstructionFile(t, target, file)
		}
	}
	expectHook := target != installers.TargetCodex
	if (expectHook && !sawHook) || !sawInstallDoc || !sawInstruction {
		t.Fatalf("missing generated files for %s", target)
	}
}

func assertInstallHookFile(t *testing.T, file installers.File) {
	t.Helper()
	if file.Strategy != installers.StrategyWrite || file.Mode != 0o755 {
		t.Fatalf("unexpected hook file metadata: %#v", file)
	}
	if strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) {
		if !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--hook claude") {
			t.Fatalf("unexpected global hook content: %q", file.Content)
		}
		return
	}
	if strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")) {
		if !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--hook cursor") {
			t.Fatalf("unexpected cursor hook content: %q", file.Content)
		}
		return
	}
	if strings.HasSuffix(file.Path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")) {
		if !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--hook gemini") {
			t.Fatalf("unexpected gemini hook content: %q", file.Content)
		}
		return
	}
	if !strings.Contains(file.Content, "./dev/szr") || !strings.Contains(file.Content, "szr hint") || !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--format hint") {
		t.Fatalf("unexpected hook content: %q", file.Content)
	}
}

func assertInstallDocFile(t *testing.T, file installers.File, target installers.Target) {
	t.Helper()
	if target == installers.TargetCodex {
		if strings.Contains(file.Content, "Hook command:") {
			t.Fatalf("unexpected codex install doc: %q", file.Content)
		}
		if !strings.Contains(file.Content, ".codex/szr.md") || !strings.Contains(file.Content, "Codex does not install a Bash rewrite hook today") || !strings.Contains(file.Content, "proxy git diff ... -- path/to/file | tail -80") || !strings.Contains(file.Content, "szr find <path> --name \"*.py\"") {
			t.Fatalf("unexpected codex install doc: %q", file.Content)
		}
		return
	}
	if !strings.Contains(file.Content, "Hook command:") {
		t.Fatalf("unexpected install doc: %q", file.Content)
	}
	if target == installers.TargetClaude {
		if !strings.Contains(file.Content, "./.claude/hooks/szr-rewrite.sh") {
			t.Fatalf("unexpected claude install doc: %q", file.Content)
		}
		return
	}
	if target == installers.TargetCursor {
		if !strings.Contains(file.Content, "./.cursor/hooks/szr-rewrite.sh") {
			t.Fatalf("unexpected cursor install doc: %q", file.Content)
		}
		return
	}
	if target == installers.TargetGemini {
		if !strings.Contains(file.Content, "./.gemini/hooks/szr-rewrite.sh") {
			t.Fatalf("unexpected gemini install doc: %q", file.Content)
		}
		return
	}
	if !strings.Contains(file.Content, "./.szr/hooks/pre-command.sh") {
		t.Fatalf("unexpected install doc: %q", file.Content)
	}
}

func assertInstallInstructionFile(t *testing.T, target installers.Target, file installers.File) {
	t.Helper()
	switch target {
	case installers.TargetCodex:
		if strings.HasSuffix(file.Path, filepath.Join(".codex", "szr.md")) {
			if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Codex") || !strings.Contains(file.Content, "Codex does not install a Bash rewrite hook today") || !strings.Contains(file.Content, "szr grep <pattern> <path>") {
				t.Fatalf("unexpected Codex shared file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, "AGENTS.md") {
			if file.Strategy != installers.StrategyMerge || file.Marker != "szr-codex" || !strings.Contains(file.Content, "@") {
				t.Fatalf("unexpected Codex AGENTS.md file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected Codex instruction file: %#v", file)
	case installers.TargetClaude:
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")) {
			if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Claude Code") {
				t.Fatalf("unexpected Claude shared file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")) {
			if file.Strategy != installers.StrategyMerge || file.Marker != "szr-claude-code-global" {
				t.Fatalf("unexpected Claude CLAUDE.md file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")) {
			if file.Strategy != installers.StrategyClaudeSettingsMerge {
				t.Fatalf("unexpected Claude settings file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected Claude instruction file: %#v", file)
	case installers.TargetCursor:
		if strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks.json")) {
			if file.Strategy != installers.StrategyCursorHooksMerge {
				t.Fatalf("unexpected cursor file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected cursor file: %#v", file)
	case installers.TargetGemini:
		if strings.HasSuffix(file.Path, filepath.Join(".gemini", "settings.json")) {
			if file.Strategy != installers.StrategyGeminiSettingsMerge {
				t.Fatalf("unexpected gemini file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected gemini file: %#v", file)
	case installers.TargetShell:
		if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "szr_explain()") || !strings.Contains(file.Content, "szr_proxy()") {
			t.Fatalf("unexpected shell file: %#v", file)
		}
	default:
		if file.Strategy != installers.StrategyMerge || file.Marker == "" {
			t.Fatalf("unexpected merge file: %#v", file)
		}
		if !strings.Contains(file.Content, "explain <cmd...>") || !strings.Contains(file.Content, "proxy <cmd...>") || !strings.Contains(file.Content, "wrap the noisy producer instead of the whole pipeline") || !strings.Contains(file.Content, "szr find <path> --name \"*.py\"") || !strings.Contains(file.Content, "szr run /usr/bin/grep ...") {
			t.Fatalf("unexpected instruction body: %q", file.Content)
		}
	}
}

func TestRenderUninstallTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(root, "go.mod"), "module github.com/devr-tools/szr\n")
	testutil.MustWriteFile(t, filepath.Join(root, "cmd", "szr", "main.go"), "package main\n")
	home := t.TempDir()

	plans, err := installers.RenderAllUninstall(installers.Options{
		RepoRoot: root,
		Binary:   "./dev/szr",
		HomeDir:  home,
	})
	if err != nil {
		t.Fatalf("render all uninstall: %v", err)
	}
	if len(plans) != 5 {
		t.Fatalf("unexpected uninstall plan count: %d", len(plans))
	}

	for _, target := range installers.Targets() {
		t.Run(string(target), func(t *testing.T) {
			plan, err := installers.RenderUninstall(target, installers.Options{
				RepoRoot: root,
				Binary:   "./dev/szr",
				HomeDir:  home,
			})
			if err != nil {
				t.Fatalf("render uninstall %s: %v", target, err)
			}
			assertUninstallRenderPlan(t, target, plan)
		})
	}

	if _, err := installers.RenderUninstall("unknown", installers.Options{RepoRoot: root}); err == nil {
		t.Fatal("expected unknown uninstall target error")
	}
}

func TestRenderClaudeUninstallTarget(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	plan, err := installers.RenderUninstall(installers.TargetClaude, installers.Options{
		HomeDir: home,
		Binary:  "szr",
	})
	if err != nil {
		t.Fatalf("render claude uninstall: %v", err)
	}
	if !plan.Paths.Global {
		t.Fatalf("expected global uninstall plan metadata: %#v", plan.Paths)
	}

	var sawHook, sawInstallDoc, sawSZRDoc, sawClaudeMD, sawSettings bool
	for _, file := range plan.Files {
		switch {
		case strings.HasSuffix(file.Path, filepath.Join(".claude", ".szr", "install", "claude-code.md")):
			sawInstallDoc = true
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")):
			sawHook = true
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected global uninstall hook file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")):
			sawSZRDoc = true
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected global uninstall szr.md file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")):
			sawClaudeMD = true
			if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-claude-code-global" {
				t.Fatalf("unexpected global uninstall CLAUDE.md file: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")):
			sawSettings = true
			if file.Strategy != installers.StrategyClaudeSettingsPrune {
				t.Fatalf("unexpected global uninstall settings file: %#v", file)
			}
		}
	}
	if !sawHook || !sawInstallDoc || !sawSZRDoc || !sawClaudeMD || !sawSettings {
		t.Fatalf("missing generated files for claude uninstall: %#v", plan.Files)
	}
}

func assertUninstallRenderPlan(t *testing.T, target installers.Target, plan installers.Plan) {
	t.Helper()
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
	} else if target == installers.TargetCodex {
		if !strings.Contains(plan.ManualSteps[0], ".codex/szr.md") {
			t.Fatalf("unexpected codex uninstall manual step: %q", plan.ManualSteps[0])
		}
	} else if target == installers.TargetClaude {
		if !strings.Contains(plan.ManualSteps[0], ".claude/settings.json") {
			t.Fatalf("unexpected claude uninstall manual step: %q", plan.ManualSteps[0])
		}
	} else if target == installers.TargetCursor {
		if !strings.Contains(plan.ManualSteps[0], ".cursor/hooks.json") {
			t.Fatalf("unexpected cursor uninstall manual step: %q", plan.ManualSteps[0])
		}
	} else if target == installers.TargetGemini {
		if !strings.Contains(plan.ManualSteps[0], ".gemini/settings.json") {
			t.Fatalf("unexpected gemini uninstall manual step: %q", plan.ManualSteps[0])
		}
	} else if !strings.Contains(plan.ManualSteps[0], ".szr/hooks/pre-command.sh") {
		t.Fatalf("unexpected uninstall manual step: %q", plan.ManualSteps[0])
	}

	var sawInstallDoc, sawTargetFile bool
	for _, file := range plan.Files {
		switch {
		case strings.HasSuffix(file.Path, filepath.Join(".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".claude", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".codex", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".cursor", ".szr", "install", string(target)+".md")) ||
			strings.HasSuffix(file.Path, filepath.Join(".gemini", ".szr", "install", string(target)+".md")):
			sawInstallDoc = true
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected uninstall doc metadata: %#v", file)
			}
		case strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")) ||
			strings.HasSuffix(file.Path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")):
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected uninstall hook metadata: %#v", file)
			}
		default:
			sawTargetFile = true
			assertUninstallInstructionFile(t, target, file)
		}
	}
	expectHook := targetUsesHookFile(target)
	if !sawInstallDoc || !sawTargetFile {
		t.Fatalf("missing uninstall files for %s", target)
	}
	if expectHook {
		foundHook := false
		for _, file := range plan.Files {
			if strings.HasSuffix(file.Path, filepath.Join(".szr", "hooks", "pre-command.sh")) ||
				strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) ||
				strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")) ||
				strings.HasSuffix(file.Path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")) {
				foundHook = true
				break
			}
		}
		if !foundHook {
			t.Fatalf("missing uninstall hook for %s", target)
		}
	}
}

func targetUsesHookFile(target installers.Target) bool {
	return target != installers.TargetCodex
}

func assertUninstallInstructionFile(t *testing.T, target installers.Target, file installers.File) {
	t.Helper()
	switch target {
	case installers.TargetCodex:
		if strings.HasSuffix(file.Path, filepath.Join(".codex", "szr.md")) {
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected uninstall codex szr.md file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, "AGENTS.md") {
			if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-codex" {
				t.Fatalf("unexpected uninstall codex AGENTS file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected uninstall codex file: %#v", file)
	case installers.TargetClaude:
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")) {
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected uninstall claude szr.md file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")) {
			if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-claude-code-global" {
				t.Fatalf("unexpected uninstall claude CLAUDE.md file: %#v", file)
			}
			return
		}
		if strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")) {
			if file.Strategy != installers.StrategyClaudeSettingsPrune {
				t.Fatalf("unexpected uninstall claude settings file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected uninstall claude file: %#v", file)
	case installers.TargetCursor:
		if strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks.json")) {
			if file.Strategy != installers.StrategyCursorHooksPrune {
				t.Fatalf("unexpected uninstall cursor hooks.json file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected uninstall cursor file: %#v", file)
	case installers.TargetGemini:
		if strings.HasSuffix(file.Path, filepath.Join(".gemini", "settings.json")) {
			if file.Strategy != installers.StrategyGeminiSettingsPrune {
				t.Fatalf("unexpected uninstall gemini settings file: %#v", file)
			}
			return
		}
		t.Fatalf("unexpected uninstall gemini file: %#v", file)
	case installers.TargetShell:
		if file.Strategy != installers.StrategyDelete {
			t.Fatalf("unexpected uninstall delete file: %#v", file)
		}
	default:
		if file.Strategy != installers.StrategyUnmerge || file.Marker == "" {
			t.Fatalf("unexpected uninstall merge removal file: %#v", file)
		}
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
