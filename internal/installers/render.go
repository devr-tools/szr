package installers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Render(target Target, options Options) (Plan, error) {
	paths, err := detectRenderPaths(target, options)
	if err != nil {
		return Plan{}, err
	}
	if options.Binary != "" {
		paths.Binary = options.Binary
	}

	switch target {
	case TargetCodex:
		return renderCodex(paths), nil
	case TargetClaude:
		return renderClaude(paths), nil
	case TargetCursor:
		return renderCursor(paths), nil
	case TargetGemini:
		return renderGemini(paths), nil
	case TargetShell:
		return renderShell(paths), nil
	default:
		return Plan{}, fmt.Errorf("unknown target %q", target)
	}
}

func RenderAll(options Options) ([]Plan, error) {
	targets := Targets()
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		plan, err := Render(target, options)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func detectRenderPaths(target Target, options Options) (Paths, error) {
	homeDir := options.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
	}

	switch target {
	case TargetClaude, TargetCursor, TargetGemini:
		return DetectClaudeGlobalPaths(homeDir)
	case TargetCodex:
		repoPaths, err := DetectPaths(options.RepoRoot)
		if err != nil {
			return Paths{}, err
		}
		globalPaths, err := DetectClaudeGlobalPaths(homeDir)
		if err != nil {
			return Paths{}, err
		}
		repoPaths.CodexDir = globalPaths.CodexDir
		repoPaths.CodexSZRFile = globalPaths.CodexSZRFile
		return repoPaths, nil
	default:
		return DetectPaths(options.RepoRoot)
	}
}

func sharedHookFile(paths Paths) File {
	return File{
		Path:        paths.HookFile,
		Content:     renderSharedHookScript(paths.HookFile, paths.Binary),
		Mode:        0o755,
		Strategy:    StrategyWrite,
		Description: "pre-command reminder hook",
	}
}

func sharedInstallDoc(paths Paths, target Target, title, instructionPath string) File {
	return File{
		Path:        installDocPath(paths, target),
		Content:     renderInstallDoc(paths, target, title, instructionPath),
		Mode:        0o644,
		Strategy:    StrategyWrite,
		Description: "manual install guide",
	}
}

func installDocPath(paths Paths, target Target) string {
	switch target {
	case TargetCodex:
		return filepath.Join(paths.CodexDir, ".szr", "install", string(target)+".md")
	case TargetCursor:
		return filepath.Join(paths.CursorDir, ".szr", "install", string(target)+".md")
	case TargetGemini:
		return filepath.Join(paths.GeminiDir, ".szr", "install", string(target)+".md")
	default:
		return filepath.Join(paths.InstallDir, string(target)+".md")
	}
}

func renderHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
  exit 0
fi

if [ "$1" = "szr" ]; then
  exit 0
fi

hint=$(%s rewrite --binary %q --format hint --command "$*" 2>/dev/null || true)
if [ -n "$hint" ]; then
  printf 'szr hint: prefer %%s\n' "$hint" >&2
fi
`, binary, binary)
}

func renderSharedHookScript(path, binary string) string {
	switch filepath.Base(path) {
	case "szr-rewrite.sh":
		if strings.Contains(filepath.ToSlash(path), "/.cursor/") {
			return renderCursorHookScript(binary)
		}
		if strings.Contains(filepath.ToSlash(path), "/.gemini/") {
			return renderGeminiHookScript(binary)
		}
		return renderClaudeGlobalHookScript(binary)
	default:
		return renderHookScript(binary)
	}
}

func renderClaudeGlobalHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook claude || true
`, binary, binary)
}

func renderCursorHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook cursor || printf '{}'
`, binary, binary)
}

func renderGeminiHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

%s rewrite --binary %q --hook gemini || printf '{"decision":"allow"}'
`, binary, binary)
}

func renderInstructionBody(paths Paths) string {
	return renderInstructionBodyForHook(paths, paths.HookFile)
}

func renderInstructionBodyForHook(paths Paths, hookPath string) string {
	hookDescription := fmt.Sprintf("- The repository-local reminder hook lives at `%s`.", relativePath(paths.RepoRoot, hookPath))
	if paths.Global {
		hookDescription = fmt.Sprintf("- The installed szr agent hook lives at `%s`.", relativePath(paths.RepoRoot, hookPath))
	}
	lines := []string{
		"Use `szr` as the default entrypoint for noisy repository commands.",
		"",
		fmt.Sprintf("- Prefer `%s git status`, `%s git diff`, `%s git log`, and `%s go test ./...` over raw shell commands.", paths.Binary, paths.Binary, paths.Binary, paths.Binary),
		fmt.Sprintf("- When using pipes or redirection, wrap the noisy producer instead of the whole pipeline, for example `%s proxy git diff --stat HEAD~1..HEAD 2>&1 | tail -80`.", paths.Binary),
		fmt.Sprintf("- For git diff review loops, use `%s git diff ... --stat` or `%s proxy git diff ... -- path/to/file | head -200` rather than a raw piped `git diff` call.", paths.Binary, paths.Binary),
		fmt.Sprintf("- Prefer `%s find <path> --name \"*.py\"` for file discovery, and `%s grep <pattern> <path>` or `%s rg <pattern> <path>` for grouped code search.", paths.Binary, paths.Binary, paths.Binary),
		fmt.Sprintf("- When exact `/usr/bin/find` or `/usr/bin/grep` flags matter, wrap them explicitly with `%s run /usr/bin/find ...` or `%s run /usr/bin/grep ...` instead of expecting an auto-rewrite.", paths.Binary, paths.Binary),
		fmt.Sprintf("- Use `%s explain <cmd...>` when you need to inspect the active profile before bypassing it.", paths.Binary),
		fmt.Sprintf("- Use `%s proxy <cmd...>` when raw output matters more than compression.", paths.Binary),
		"- If `szr` reports a tee artifact for a failure, inspect that full artifact path instead of rerunning the command unfiltered.",
		hookDescription,
	}
	return strings.Join(lines, "\n")
}

func renderCodexInstructionBody(paths Paths) string {
	lines := []string{
		"Use `szr` as the default entrypoint for noisy repository commands.",
		"",
		fmt.Sprintf("- Prefer `%s git status`, `%s git diff`, `%s git log`, and `%s go test ./...` over raw shell commands.", paths.Binary, paths.Binary, paths.Binary, paths.Binary),
		fmt.Sprintf("- Codex does not install a Bash rewrite hook today, so tool calls must invoke `%s` explicitly.", paths.Binary),
		fmt.Sprintf("- When using pipes, redirection, or absolute-path binaries, keep `%s` on the noisy command itself, for example `%s proxy git diff --stat HEAD~1..HEAD 2>&1 | head -200`.", paths.Binary, paths.Binary),
		fmt.Sprintf("- For git diff review loops, prefer `%s git diff ... --stat` and `%s proxy git diff ... -- path/to/file | tail -80` instead of raw piped `git diff` calls.", paths.Binary, paths.Binary),
		fmt.Sprintf("- Prefer `%s find <path> --name \"*.py\"` for file discovery, and `%s grep <pattern> <path>` or `%s rg <pattern> <path>` for grouped code search.", paths.Binary, paths.Binary, paths.Binary),
		fmt.Sprintf("- If exact `/usr/bin/find` or `/usr/bin/grep` flags matter, wrap them explicitly with `%s run /usr/bin/find ...` or `%s run /usr/bin/grep ...`.", paths.Binary, paths.Binary),
		fmt.Sprintf("- Use `%s explain <cmd...>` when you need to inspect the active profile before bypassing it.", paths.Binary),
		fmt.Sprintf("- Use `%s proxy <cmd...>` when raw output matters more than compression.", paths.Binary),
		"- If `szr` reports a tee artifact for a failure, inspect that full artifact path instead of rerunning the command unfiltered.",
	}
	return strings.Join(lines, "\n")
}

func renderInstallDoc(paths Paths, target Target, title, instructionPath string) string {
	hookPath := paths.HookFile
	scopeNote := "This installer is repo-local on purpose. Keep the generated instruction file committed, and wire the hook command into your " + string(target) + " shell/tool configuration separately.\n"
	switch target {
	case TargetClaude:
		scopeNote = "This installer patches your Claude Code home directory. Keep the generated hook and `szr.md` in place, then restart Claude Code so it reloads the hook registration.\n"
		hookPath = paths.HookFile
	case TargetCodex:
		return fmt.Sprintf(
			"# %s installer\n\n"+
				"Instruction file: %s\n"+
				"Repo reference: %s\n\n"+
				"Suggested repo policy:\n\n"+
				"%s\n\n"+
				"This installer writes shared Codex guidance under your Codex home directory and references it from the repo AGENTS.md file.\n",
			title,
			relativePath(paths.RepoRoot, instructionPath),
			relativePath(paths.RepoRoot, filepath.Join(paths.RepoRoot, "AGENTS.md")),
			renderCodexInstructionBody(paths),
		)
	case TargetCursor:
		scopeNote = "This installer patches your Cursor hooks.json and installs a preToolUse hook script under ~/.cursor/hooks/.\n"
		hookPath = paths.CursorHookFile
	case TargetGemini:
		scopeNote = "This installer patches your Gemini CLI settings.json and installs a BeforeTool hook script under ~/.gemini/hooks/.\n"
		hookPath = paths.GeminiHookFile
	}
	return fmt.Sprintf(
		"# %s installer\n\n"+
			"Instruction file: %s\n"+
			"Hook script: %s\n\n"+
			"Hook command:\n\n"+
			"```sh\n"+
			"%s \"$@\"\n"+
			"```\n\n"+
			"Suggested repo policy:\n\n"+
			"%s\n\n"+
			"%s",
		title,
		relativePath(paths.RepoRoot, instructionPath),
		relativePath(paths.RepoRoot, hookPath),
		relativePath(paths.RepoRoot, hookPath),
		renderInstructionBodyForHook(paths, hookPath),
		scopeNote,
	)
}

func relativePath(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return "./" + strings.TrimPrefix(filepath.ToSlash(rel), "./")
}
