package installers

import (
	"fmt"
	"os"
	"path/filepath"
)

func RenderUninstall(target Target, options Options) (Plan, error) {
	paths, err := detectRenderPaths(target, options)
	if err != nil {
		return Plan{}, err
	}
	if options.Binary != "" {
		paths.Binary = options.Binary
	}

	files, manualSteps, err := renderUninstallFiles(target, paths, false)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Target:      target,
		Title:       fmt.Sprintf("%s uninstall", target),
		Paths:       paths,
		Files:       files,
		ManualSteps: manualSteps,
	}, nil
}

func RenderAllUninstall(options Options) ([]Plan, error) {
	targets := Targets()
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
		paths, err := detectRenderPaths(target, options)
		if err != nil {
			return nil, err
		}
		if options.Binary != "" {
			paths.Binary = options.Binary
		}
		files, manualSteps, err := renderUninstallFiles(target, paths, true)
		if err != nil {
			return nil, err
		}
		plans = append(plans, Plan{
			Target:      target,
			Title:       fmt.Sprintf("%s uninstall", target),
			Paths:       paths,
			Files:       files,
			ManualSteps: manualSteps,
		})
	}
	return plans, nil
}

func renderUninstallFiles(target Target, paths Paths, allTargets bool) ([]File, []string, error) {
	if target == TargetCodex && paths.Global {
		return []File{{
				Path:        filepath.Join(paths.CodexDir, "AGENTS.md"),
				Mode:        0o644,
				Strategy:    StrategyUnmerge,
				Marker:      "szr-codex-global",
				Description: "remove Codex global instructions",
			}}, []string{
				fmt.Sprintf("Restart Codex after removing `%s` if it was already running.", codexGlobalDisplayPath(paths)),
			}, nil
	}
	files := make([]File, 0, 3)
	files = append(files, uninstallInstallDoc(paths, target))
	if targetUsesSharedHook(target) && (allTargets || shouldRemoveSharedHook(paths, target)) {
		files = append(files, uninstallHookFile(paths))
	}

	switch target {
	case TargetCodex:
		files = append(files,
			File{
				Path:        paths.CodexSZRFile,
				Strategy:    StrategyDelete,
				Description: "remove Codex shared instructions",
			},
			File{
				Path:        filepath.Join(paths.RepoRoot, "AGENTS.md"),
				Mode:        0o644,
				Strategy:    StrategyUnmerge,
				Marker:      "szr-codex",
				Description: "remove Codex repo instructions",
			},
		)
		return files, []string{
			fmt.Sprintf("Restart Codex after removing `%s` if it was already running.", codexDisplayPath(paths)),
		}, nil
	case TargetClaude:
		files = append(files,
			File{
				Path:        paths.ClaudeSZRFile,
				Strategy:    StrategyDelete,
				Description: "remove Claude Code shared instructions",
			},
			File{
				Path:        paths.ClaudeMDFile,
				Mode:        0o644,
				Strategy:    StrategyUnmerge,
				Marker:      "szr-claude-code-global",
				Description: "remove Claude Code global CLAUDE.md reference",
			},
			File{
				Path:        paths.ClaudeConfig,
				Content:     paths.HookFile,
				Mode:        0o644,
				Strategy:    StrategyClaudeSettingsPrune,
				Description: "remove Claude Code hook registration",
			},
		)
		return files, []string{
			fmt.Sprintf("Restart Claude Code after removing `%s` so it drops the szr PreToolUse hook.", relativePath(paths.RepoRoot, paths.ClaudeConfig)),
		}, nil
	case TargetCursor:
		files = append(files,
			File{
				Path:        paths.CursorHookFile,
				Strategy:    StrategyDelete,
				Description: "remove Cursor hook script",
			},
			File{
				Path:        paths.CursorConfig,
				Content:     paths.CursorHookFile,
				Mode:        0o644,
				Strategy:    StrategyCursorHooksPrune,
				Description: "remove Cursor hook registration",
			},
		)
		return files, []string{
			fmt.Sprintf("Restart Cursor after removing `%s` so it drops the preToolUse hook.", relativePath(paths.RepoRoot, paths.CursorConfig)),
		}, nil
	case TargetGemini:
		files = append(files,
			File{
				Path:        paths.GeminiHookFile,
				Strategy:    StrategyDelete,
				Description: "remove Gemini hook script",
			},
			File{
				Path:        paths.GeminiConfig,
				Content:     paths.GeminiHookFile,
				Mode:        0o644,
				Strategy:    StrategyGeminiSettingsPrune,
				Description: "remove Gemini hook registration",
			},
		)
		return files, []string{
			fmt.Sprintf("Restart Gemini CLI after removing `%s` so it drops the BeforeTool hook.", relativePath(paths.RepoRoot, paths.GeminiConfig)),
		}, nil
	case TargetShell:
		snippetPath := filepath.Join(paths.InstallDir, "shell.sh")
		files = append(files, File{
			Path:        snippetPath,
			Strategy:    StrategyDelete,
			Description: "remove shell bootstrap snippet",
		})
		return files, []string{
			fmt.Sprintf("Remove any shell rc sourcing line that still references `%s`.", relativePath(paths.RepoRoot, snippetPath)),
		}, nil
	default:
		return nil, nil, fmt.Errorf("unknown target %q", target)
	}
}

func uninstallHookFile(paths Paths) File {
	return File{
		Path:        paths.HookFile,
		Strategy:    StrategyDelete,
		Description: "remove pre-command reminder hook",
	}
}

func uninstallInstallDoc(paths Paths, target Target) File {
	return File{
		Path:        installDocPath(paths, target),
		Strategy:    StrategyDelete,
		Description: "remove manual install guide",
	}
}

func shouldRemoveSharedHook(paths Paths, removing Target) bool {
	for _, target := range Targets() {
		if target == removing {
			continue
		}
		if !targetUsesSharedHook(target) {
			continue
		}
		if _, err := os.Stat(installDocPath(paths, target)); err == nil {
			return false
		}
	}
	return true
}

func targetUsesSharedHook(target Target) bool {
	return target == TargetClaude || target == TargetShell
}
