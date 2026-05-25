package installers

import (
	"fmt"
	"os"
	"path/filepath"
)

func RenderUninstall(target Target, options Options) (Plan, error) {
	paths, err := DetectPaths(options.RepoRoot)
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
	paths, err := DetectPaths(options.RepoRoot)
	if err != nil {
		return nil, err
	}
	if options.Binary != "" {
		paths.Binary = options.Binary
	}

	targets := Targets()
	plans := make([]Plan, 0, len(targets))
	for _, target := range targets {
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
	files := make([]File, 0, 3)
	files = append(files, uninstallInstallDoc(paths, target))
	if allTargets || shouldRemoveSharedHook(paths, target) {
		files = append(files, uninstallHookFile(paths))
	}

	switch target {
	case TargetCodex:
		instructionPath := filepath.Join(paths.RepoRoot, "AGENTS.md")
		files = append(files, File{
			Path:        instructionPath,
			Mode:        0o644,
			Strategy:    StrategyUnmerge,
			Marker:      "szr-codex",
			Description: "remove Codex repo instructions",
		})
		return files, []string{
			fmt.Sprintf("Remove any Codex pre-command hook entry that still references `%s`.", relativePath(paths.RepoRoot, paths.HookFile)),
		}, nil
	case TargetClaude:
		instructionPath := filepath.Join(paths.RepoRoot, "CLAUDE.md")
		files = append(files, File{
			Path:        instructionPath,
			Mode:        0o644,
			Strategy:    StrategyUnmerge,
			Marker:      "szr-claude-code",
			Description: "remove Claude Code repo instructions",
		})
		return files, []string{
			fmt.Sprintf("Remove any Claude Code pre-command hook entry that still references `%s`.", relativePath(paths.RepoRoot, paths.HookFile)),
		}, nil
	case TargetCursor:
		rulePath := filepath.Join(paths.CursorRuleDir, "szr.mdc")
		files = append(files, File{
			Path:        rulePath,
			Strategy:    StrategyDelete,
			Description: "remove Cursor rule",
		})
		return files, []string{
			fmt.Sprintf("Remove any Cursor pre-command reminder that still references `%s`.", relativePath(paths.RepoRoot, paths.HookFile)),
		}, nil
	case TargetGemini:
		instructionPath := filepath.Join(paths.RepoRoot, "GEMINI.md")
		files = append(files, File{
			Path:        instructionPath,
			Mode:        0o644,
			Strategy:    StrategyUnmerge,
			Marker:      "szr-gemini",
			Description: "remove Gemini repo instructions",
		})
		return files, []string{
			fmt.Sprintf("Remove any Gemini pre-command hook entry that still references `%s`.", relativePath(paths.RepoRoot, paths.HookFile)),
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
		if _, err := os.Stat(installDocPath(paths, target)); err == nil {
			return false
		}
	}
	return true
}
