package installers

import (
	"fmt"
	"path/filepath"
	"strings"
)

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
