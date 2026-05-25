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

hint=""

case "$1" in
  git)
    case "${2:-}" in
      status|diff|log|show)
        hint="%s git ${2:-}"
        ;;
    esac
    ;;
  go)
    case "${2:-}" in
      test|build|vet|list)
        hint="%s go ${2:-}"
        ;;
    esac
    ;;
  npm|pnpm|yarn|bun)
    hint="%s $*"
    ;;
  pytest|cargo|docker)
    hint="%s $*"
    ;;
esac

if [ -n "$hint" ]; then
  printf 'szr hint: prefer %%s\n' "$hint" >&2
fi
`, binary, binary, binary, binary)
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

if ! command -v python3 >/dev/null 2>&1; then
  exit 0
fi

python3 -c '
import json
import shlex
import sys

binary = %q

def rewrite(command):
    try:
        parts = shlex.split(command)
    except ValueError:
        return command
    if not parts:
        return command
    if parts[0] == binary:
        return command
    if parts[0] in {"git", "npm", "pnpm", "yarn", "bun", "pytest", "cargo", "docker"}:
        return binary + " " + command
    if parts[0] == "go" and len(parts) > 1 and parts[1] in {"test", "build", "vet", "list"}:
        return binary + " " + command
    return command

try:
    payload = json.load(sys.stdin)
except Exception:
    sys.exit(0)

tool_name = payload.get("tool_name")
tool_input = payload.get("tool_input")
if tool_name != "Bash" or not isinstance(tool_input, dict):
    sys.exit(0)

command = tool_input.get("command")
if not isinstance(command, str):
    sys.exit(0)

updated = rewrite(command)
if updated == command:
    sys.exit(0)

json.dump({
    "hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "allow",
        "permissionDecisionReason": "szr auto-rewrite",
        "updatedInput": {"command": updated},
    }
}, sys.stdout)
' || true
`, binary)
}

func renderCursorHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

if ! command -v python3 >/dev/null 2>&1; then
  printf '{}'
  exit 0
fi

python3 -c '
import json
import shlex
import sys

binary = %q

def rewrite(command):
    try:
        parts = shlex.split(command)
    except ValueError:
        return command
    if not parts:
        return command
    if parts[0] == binary:
        return command
    if parts[0] in {"git", "npm", "pnpm", "yarn", "bun", "pytest", "cargo", "docker"}:
        return binary + " " + command
    if parts[0] == "go" and len(parts) > 1 and parts[1] in {"test", "build", "vet", "list"}:
        return binary + " " + command
    return command

try:
    payload = json.load(sys.stdin)
except Exception:
    print("{}")
    sys.exit(0)

tool_name = payload.get("tool_name")
tool_input = payload.get("tool_input")
if tool_name != "Bash" or not isinstance(tool_input, dict):
    print("{}")
    sys.exit(0)

command = tool_input.get("command")
if not isinstance(command, str):
    print("{}")
    sys.exit(0)

updated = rewrite(command)
if updated == command:
    print("{}")
    sys.exit(0)

json.dump({
    "permission": "allow",
    "updated_input": {"command": updated},
}, sys.stdout)
' || printf '{}'
`, binary)
}

func renderGeminiHookScript(binary string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu

if ! command -v python3 >/dev/null 2>&1; then
  printf '{"decision":"allow"}'
  exit 0
fi

python3 -c '
import json
import shlex
import sys

binary = %q

def rewrite(command):
    try:
        parts = shlex.split(command)
    except ValueError:
        return command
    if not parts:
        return command
    if parts[0] == binary:
        return command
    if parts[0] in {"git", "npm", "pnpm", "yarn", "bun", "pytest", "cargo", "docker"}:
        return binary + " " + command
    if parts[0] == "go" and len(parts) > 1 and parts[1] in {"test", "build", "vet", "list"}:
        return binary + " " + command
    return command

try:
    payload = json.load(sys.stdin)
except Exception:
    print("{\"decision\":\"allow\"}")
    sys.exit(0)

tool_name = payload.get("tool_name")
tool_input = payload.get("tool_input")
if tool_name != "run_shell_command" or not isinstance(tool_input, dict):
    print("{\"decision\":\"allow\"}")
    sys.exit(0)

command = tool_input.get("command")
if not isinstance(command, str):
    print("{\"decision\":\"allow\"}")
    sys.exit(0)

updated = rewrite(command)
if updated == command:
    print("{\"decision\":\"allow\"}")
    sys.exit(0)

json.dump({
    "decision": "allow",
    "hookSpecificOutput": {
        "tool_input": {"command": updated},
    }
}, sys.stdout)
' || printf '{"decision":"allow"}'
`, binary)
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
