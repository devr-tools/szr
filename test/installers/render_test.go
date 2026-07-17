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

	saw := map[string]bool{}
	for _, file := range plan.Files {
		key := classifyClaudeInstallFile(file.Path)
		saw[key] = true
		assertClaudeInstallFile(t, key, file)
	}
	if !saw["hook"] || !saw["install-doc"] || !saw["szr"] || !saw["claude-md"] || !saw["settings"] {
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
		switch classifyInstallPlanFile(target, file.Path) {
		case "hook":
			sawHook = true
			assertInstallHookFile(t, file)
		case "install-doc":
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
	wantHook := ""
	switch {
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")):
		wantHook = "--hook claude"
	case strings.HasSuffix(file.Path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")):
		wantHook = "--hook cursor"
	case strings.HasSuffix(file.Path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")):
		wantHook = "--hook gemini"
	}
	if wantHook != "" {
		if !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, wantHook) {
			t.Fatalf("unexpected hook content: %q", file.Content)
		}
		return
	}
	if !strings.Contains(file.Content, "./dev/szr") || !strings.Contains(file.Content, "szr hint") || !strings.Contains(file.Content, "rewrite --binary") || !strings.Contains(file.Content, "--format hint") {
		t.Fatalf("unexpected hook content: %q", file.Content)
	}
}

func assertInstallDocFile(t *testing.T, file installers.File, target installers.Target) {
	t.Helper()
	assertCommandChoiceGuidance(t, file.Content, "./dev/szr")
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
		assertInstallCodexInstruction(t, file)
	case installers.TargetClaude:
		assertInstallClaudeInstruction(t, file)
	case installers.TargetCursor:
		assertExactStrategySuffix(t, file, filepath.Join(".cursor", "hooks.json"), installers.StrategyCursorHooksMerge, "cursor")
	case installers.TargetGemini:
		assertExactStrategySuffix(t, file, filepath.Join(".gemini", "settings.json"), installers.StrategyGeminiSettingsMerge, "gemini")
	case installers.TargetShell:
		assertInstallShellInstruction(t, file)
	default:
		assertInstallDefaultInstruction(t, file)
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

	saw := map[string]bool{}
	for _, file := range plan.Files {
		key := classifyClaudeUninstallFile(file.Path)
		saw[key] = true
		assertClaudeUninstallFile(t, key, file)
	}
	if !saw["hook"] || !saw["install-doc"] || !saw["szr"] || !saw["claude-md"] || !saw["settings"] {
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
	if !strings.Contains(plan.ManualSteps[0], uninstallManualStepSubstring(target)) {
		t.Fatalf("unexpected uninstall manual step: %q", plan.ManualSteps[0])
	}

	var sawInstallDoc, sawTargetFile bool
	for _, file := range plan.Files {
		switch classifyInstallPlanFile(target, file.Path) {
		case "install-doc":
			sawInstallDoc = true
			if file.Strategy != installers.StrategyDelete {
				t.Fatalf("unexpected uninstall doc metadata: %#v", file)
			}
		case "hook":
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
	if expectHook && !planHasHookFile(target, plan.Files) {
		t.Fatalf("missing uninstall hook for %s", target)
	}
}

func targetUsesHookFile(target installers.Target) bool {
	return target != installers.TargetCodex
}

func assertUninstallInstructionFile(t *testing.T, target installers.Target, file installers.File) {
	t.Helper()
	switch target {
	case installers.TargetCodex:
		assertUninstallCodexInstruction(t, file)
	case installers.TargetClaude:
		assertUninstallClaudeInstruction(t, file)
	case installers.TargetCursor:
		assertExactStrategySuffix(t, file, filepath.Join(".cursor", "hooks.json"), installers.StrategyCursorHooksPrune, "uninstall cursor")
	case installers.TargetGemini:
		assertExactStrategySuffix(t, file, filepath.Join(".gemini", "settings.json"), installers.StrategyGeminiSettingsPrune, "uninstall gemini")
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

func assertInstallShellInstruction(t *testing.T, file installers.File) {
	t.Helper()
	if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "szr_explain()") || !strings.Contains(file.Content, "szr_proxy()") {
		t.Fatalf("unexpected shell file: %#v", file)
	}
}

func assertInstallDefaultInstruction(t *testing.T, file installers.File) {
	t.Helper()
	if file.Strategy != installers.StrategyMerge || file.Marker == "" {
		t.Fatalf("unexpected merge file: %#v", file)
	}
	if !strings.Contains(file.Content, "explain <cmd...>") || !strings.Contains(file.Content, "proxy <cmd...>") || !strings.Contains(file.Content, "wrap the noisy producer instead of the whole pipeline") || !strings.Contains(file.Content, "szr find <path> --name \"*.py\"") || !strings.Contains(file.Content, "szr run /usr/bin/grep ...") {
		t.Fatalf("unexpected instruction body: %q", file.Content)
	}
	if !strings.Contains(file.Content, "Orchestrators export `SZR_SESSION=<id>` so parallel agents share references") || !strings.Contains(file.Content, "since last run") {
		t.Fatalf("expected delta/scope guidance in instruction body: %q", file.Content)
	}
}

func assertCommandChoiceGuidance(t *testing.T, content, binary string) {
	t.Helper()
	for _, want := range []string{
		"`" + binary + " <command...>` by default for normal agent inspection",
		"Always use `" + binary + " git ...` for Git operations, including managing branches and worktrees, checking status and diffs, staging, committing, pulling, and pushing",
		"`" + binary + " proxy <command...>` is the raw-output escape hatch",
		"`" + binary + " expand <ref>` is recovery, not execution",
		"without rerunning the command",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected command-choice guidance %q in %q", want, content)
		}
	}
}

func TestTrackedCodexInstallDocGuidance(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", ".szr", "install", "codex.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tracked Codex installer doc: %v", err)
	}
	content := string(data)
	assertCommandChoiceGuidance(t, content, "./bin/szr")
	if !strings.Contains(content, "Instruction file: ~/.codex/szr.md") || !strings.Contains(content, "Repo reference: ./AGENTS.md") {
		t.Fatalf("tracked Codex installer doc still describes the legacy repo-local layout: %q", content)
	}
}

func classifyInstallPlanFile(target installers.Target, path string) string {
	if strings.HasSuffix(path, filepath.Join(".szr", "hooks", "pre-command.sh")) ||
		strings.HasSuffix(path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")) ||
		strings.HasSuffix(path, filepath.Join(".cursor", "hooks", "szr-rewrite.sh")) ||
		strings.HasSuffix(path, filepath.Join(".gemini", "hooks", "szr-rewrite.sh")) {
		return "hook"
	}
	if strings.HasSuffix(path, filepath.Join(".szr", "install", string(target)+".md")) ||
		strings.HasSuffix(path, filepath.Join(".claude", ".szr", "install", string(target)+".md")) ||
		strings.HasSuffix(path, filepath.Join(".codex", ".szr", "install", string(target)+".md")) ||
		strings.HasSuffix(path, filepath.Join(".cursor", ".szr", "install", string(target)+".md")) ||
		strings.HasSuffix(path, filepath.Join(".gemini", ".szr", "install", string(target)+".md")) {
		return "install-doc"
	}
	return "instruction"
}

func classifyClaudeInstallFile(path string) string {
	switch {
	case strings.HasSuffix(path, filepath.Join(".claude", "hooks", "szr-rewrite.sh")):
		return "hook"
	case strings.HasSuffix(path, filepath.Join(".claude", ".szr", "install", "claude-code.md")):
		return "install-doc"
	case strings.HasSuffix(path, filepath.Join(".claude", "szr.md")):
		return "szr"
	case strings.HasSuffix(path, filepath.Join(".claude", "CLAUDE.md")):
		return "claude-md"
	case strings.HasSuffix(path, filepath.Join(".claude", "settings.json")):
		return "settings"
	default:
		return ""
	}
}

func classifyClaudeUninstallFile(path string) string {
	return classifyClaudeInstallFile(path)
}

func assertClaudeInstallFile(t *testing.T, key string, file installers.File) {
	t.Helper()
	switch key {
	case "hook":
		assertInstallHookFile(t, file)
	case "install-doc":
		assertCommandChoiceGuidance(t, file.Content, "szr")
	case "szr":
		if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Claude Code") {
			t.Fatalf("unexpected global szr.md file: %#v", file)
		}
	case "claude-md":
		if file.Strategy != installers.StrategyMerge || file.Marker != "szr-claude-code-global" {
			t.Fatalf("unexpected global CLAUDE.md file: %#v", file)
		}
	case "settings":
		if file.Strategy != installers.StrategyClaudeSettingsMerge {
			t.Fatalf("unexpected global settings file: %#v", file)
		}
	default:
		t.Fatalf("unexpected global claude file: %#v", file)
	}
}

func assertClaudeUninstallFile(t *testing.T, key string, file installers.File) {
	t.Helper()
	switch key {
	case "hook", "install-doc", "szr":
		want := installers.StrategyDelete
		if key == "install-doc" {
			if file.Strategy != want {
				t.Fatalf("unexpected global uninstall install doc file: %#v", file)
			}
			return
		}
		if file.Strategy != want {
			t.Fatalf("unexpected global uninstall file: %#v", file)
		}
	case "claude-md":
		if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-claude-code-global" {
			t.Fatalf("unexpected global uninstall CLAUDE.md file: %#v", file)
		}
	case "settings":
		if file.Strategy != installers.StrategyClaudeSettingsPrune {
			t.Fatalf("unexpected global uninstall settings file: %#v", file)
		}
	default:
		t.Fatalf("unexpected global claude uninstall file: %#v", file)
	}
}

func assertInstallCodexInstruction(t *testing.T, file installers.File) {
	t.Helper()
	switch {
	case strings.HasSuffix(file.Path, filepath.Join(".codex", "szr.md")):
		assertCommandChoiceGuidance(t, file.Content, "./dev/szr")
		if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Codex") || !strings.Contains(file.Content, "Codex does not install a Bash rewrite hook today") || !strings.Contains(file.Content, "szr grep <pattern> <path>") || !strings.Contains(file.Content, "Orchestrators export `SZR_SESSION=<id>` so parallel agents share references") {
			t.Fatalf("unexpected Codex shared file: %#v", file)
		}
	case strings.HasSuffix(file.Path, "AGENTS.md"):
		if file.Strategy != installers.StrategyMerge || file.Marker != "szr-codex" || !strings.Contains(file.Content, "@") {
			t.Fatalf("unexpected Codex AGENTS.md file: %#v", file)
		}
	default:
		t.Fatalf("unexpected Codex instruction file: %#v", file)
	}
}

func assertInstallClaudeInstruction(t *testing.T, file installers.File) {
	t.Helper()
	switch {
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")):
		assertCommandChoiceGuidance(t, file.Content, "./dev/szr")
		if file.Strategy != installers.StrategyWrite || !strings.Contains(file.Content, "## szr for Claude Code") {
			t.Fatalf("unexpected Claude shared file: %#v", file)
		}
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")):
		if file.Strategy != installers.StrategyMerge || file.Marker != "szr-claude-code-global" {
			t.Fatalf("unexpected Claude CLAUDE.md file: %#v", file)
		}
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")):
		if file.Strategy != installers.StrategyClaudeSettingsMerge {
			t.Fatalf("unexpected Claude settings file: %#v", file)
		}
	default:
		t.Fatalf("unexpected Claude instruction file: %#v", file)
	}
}

func assertUninstallCodexInstruction(t *testing.T, file installers.File) {
	t.Helper()
	switch {
	case strings.HasSuffix(file.Path, filepath.Join(".codex", "szr.md")):
		if file.Strategy != installers.StrategyDelete {
			t.Fatalf("unexpected uninstall codex szr.md file: %#v", file)
		}
	case strings.HasSuffix(file.Path, "AGENTS.md"):
		if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-codex" {
			t.Fatalf("unexpected uninstall codex AGENTS file: %#v", file)
		}
	default:
		t.Fatalf("unexpected uninstall codex file: %#v", file)
	}
}

func assertUninstallClaudeInstruction(t *testing.T, file installers.File) {
	t.Helper()
	switch {
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "szr.md")):
		if file.Strategy != installers.StrategyDelete {
			t.Fatalf("unexpected uninstall claude szr.md file: %#v", file)
		}
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "CLAUDE.md")):
		if file.Strategy != installers.StrategyUnmerge || file.Marker != "szr-claude-code-global" {
			t.Fatalf("unexpected uninstall claude CLAUDE.md file: %#v", file)
		}
	case strings.HasSuffix(file.Path, filepath.Join(".claude", "settings.json")):
		if file.Strategy != installers.StrategyClaudeSettingsPrune {
			t.Fatalf("unexpected uninstall claude settings file: %#v", file)
		}
	default:
		t.Fatalf("unexpected uninstall claude file: %#v", file)
	}
}

func assertExactStrategySuffix(t *testing.T, file installers.File, suffix string, strategy installers.Strategy, label string) {
	t.Helper()
	if !strings.HasSuffix(file.Path, suffix) || file.Strategy != strategy {
		t.Fatalf("unexpected %s file: %#v", label, file)
	}
}

func uninstallManualStepSubstring(target installers.Target) string {
	switch target {
	case installers.TargetShell:
		return ".szr/install/shell.sh"
	case installers.TargetCodex:
		return ".codex/szr.md"
	case installers.TargetClaude:
		return ".claude/settings.json"
	case installers.TargetCursor:
		return ".cursor/hooks.json"
	case installers.TargetGemini:
		return ".gemini/settings.json"
	default:
		return ".szr/hooks/pre-command.sh"
	}
}

func planHasHookFile(target installers.Target, files []installers.File) bool {
	for _, file := range files {
		if classifyInstallPlanFile(target, file.Path) == "hook" {
			return true
		}
	}
	return false
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
