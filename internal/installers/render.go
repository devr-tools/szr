package installers

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Render(target Target, options Options) (Plan, error) {
	paths, err := DetectPaths(options.RepoRoot)
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

func sharedHookFile(paths Paths) File {
	return File{
		Path:        paths.HookFile,
		Content:     renderHookScript(paths.Binary),
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
	return filepath.Join(paths.InstallDir, string(target)+".md")
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

func renderInstructionBody(paths Paths) string {
	lines := []string{
		"Use `szr` as the default entrypoint for noisy repository commands.",
		"",
		fmt.Sprintf("- Prefer `%s git status`, `%s git diff`, `%s git log`, and `%s go test ./...` over raw shell commands.", paths.Binary, paths.Binary, paths.Binary, paths.Binary),
		fmt.Sprintf("- Use `%s explain <cmd...>` when you need to inspect the active profile before bypassing it.", paths.Binary),
		fmt.Sprintf("- Use `%s proxy <cmd...>` when raw output matters more than compression.", paths.Binary),
		"- If `szr` reports a tee artifact for a failure, inspect that full artifact path instead of rerunning the command unfiltered.",
		fmt.Sprintf("- The repository-local reminder hook lives at `%s`.", relativePath(paths.RepoRoot, paths.HookFile)),
	}
	return strings.Join(lines, "\n")
}

func renderInstallDoc(paths Paths, target Target, title, instructionPath string) string {
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
			"This installer is repo-local on purpose. Keep the generated instruction file committed, and wire the hook command into your %s shell/tool configuration separately.\n",
		title,
		relativePath(paths.RepoRoot, instructionPath),
		relativePath(paths.RepoRoot, paths.HookFile),
		relativePath(paths.RepoRoot, paths.HookFile),
		renderInstructionBody(paths),
		string(target),
	)
}

func relativePath(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return "./" + strings.TrimPrefix(filepath.ToSlash(rel), "./")
}
