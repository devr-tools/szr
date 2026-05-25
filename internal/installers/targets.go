package installers

import (
	"fmt"
	"os"
	"path/filepath"
)

func renderCodex(paths Paths) Plan {
	return Plan{
		Target: TargetCodex,
		Title:  "Codex installer",
		Paths:  paths,
		Files: []File{
			{
				Path:        paths.CodexSZRFile,
				Content:     "## szr for Codex\n\n" + renderCodexInstructionBody(paths),
				Mode:        0o644,
				Strategy:    StrategyWrite,
				Description: "Codex shared instructions",
			},
			sharedInstallDoc(paths, TargetCodex, "Codex", paths.CodexSZRFile),
			{
				Path:        filepath.Join(paths.RepoRoot, "AGENTS.md"),
				Content:     codexReference(paths),
				Mode:        0o644,
				Strategy:    StrategyMerge,
				Marker:      "szr-codex",
				Description: "Codex repo instructions",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Keep `%s` committed so Codex loads the szr reference for this repository.", relativePath(paths.RepoRoot, filepath.Join(paths.RepoRoot, "AGENTS.md"))),
			fmt.Sprintf("Restart Codex after creating `%s` if it was already running.", codexDisplayPath(paths)),
		},
	}
}

func renderClaude(paths Paths) Plan {
	return Plan{
		Target: TargetClaude,
		Title:  "Claude Code installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetClaude, "Claude Code", paths.ClaudeSZRFile),
			{
				Path:        paths.ClaudeSZRFile,
				Content:     "## szr for Claude Code\n\n" + renderInstructionBody(paths),
				Mode:        0o644,
				Strategy:    StrategyWrite,
				Description: "Claude Code shared instructions",
			},
			{
				Path:        paths.ClaudeMDFile,
				Content:     "@szr.md",
				Mode:        0o644,
				Strategy:    StrategyMerge,
				Marker:      "szr-claude-code-global",
				Description: "Claude Code global CLAUDE.md reference",
			},
			{
				Path:        paths.ClaudeConfig,
				Content:     paths.HookFile,
				Mode:        0o644,
				Strategy:    StrategyClaudeSettingsMerge,
				Description: "Claude Code hook registration",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Keep `%s` in place so Claude Code can import the shared szr guidance.", relativePath(paths.RepoRoot, paths.ClaudeSZRFile)),
			fmt.Sprintf("Restart Claude Code after patching `%s` so the PreToolUse hook reloads.", relativePath(paths.RepoRoot, paths.ClaudeConfig)),
		},
	}
}

func renderCursor(paths Paths) Plan {
	return Plan{
		Target: TargetCursor,
		Title:  "Cursor installer",
		Paths:  paths,
		Files: []File{
			{
				Path:        paths.CursorHookFile,
				Content:     renderSharedHookScript(paths.CursorHookFile, paths.Binary),
				Mode:        0o755,
				Strategy:    StrategyWrite,
				Description: "Cursor hook script",
			},
			sharedInstallDoc(paths, TargetCursor, "Cursor", paths.CursorConfig),
			{
				Path:        paths.CursorConfig,
				Content:     paths.CursorHookFile,
				Mode:        0o644,
				Strategy:    StrategyCursorHooksMerge,
				Description: "Cursor hook registration",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Restart Cursor after patching `%s` so the preToolUse hook reloads.", relativePath(paths.RepoRoot, paths.CursorConfig)),
			fmt.Sprintf("Cursor will return `{}` when no rewrite applies and `updated_input` when szr should wrap a Bash command."),
		},
	}
}

func renderGemini(paths Paths) Plan {
	return Plan{
		Target: TargetGemini,
		Title:  "Gemini installer",
		Paths:  paths,
		Files: []File{
			{
				Path:        paths.GeminiHookFile,
				Content:     renderSharedHookScript(paths.GeminiHookFile, paths.Binary),
				Mode:        0o755,
				Strategy:    StrategyWrite,
				Description: "Gemini hook script",
			},
			sharedInstallDoc(paths, TargetGemini, "Gemini", paths.GeminiConfig),
			{
				Path:        paths.GeminiConfig,
				Content:     paths.GeminiHookFile,
				Mode:        0o644,
				Strategy:    StrategyGeminiSettingsMerge,
				Description: "Gemini hook registration",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Restart Gemini CLI after patching `%s` so the BeforeTool hook reloads.", relativePath(paths.RepoRoot, paths.GeminiConfig)),
			fmt.Sprintf("Gemini rewrites only `run_shell_command` tool calls; direct model reads still bypass szr."),
		},
	}
}

func renderShell(paths Paths) Plan {
	snippetPath := filepath.Join(paths.InstallDir, "shell.sh")
	return Plan{
		Target: TargetShell,
		Title:  "Shell installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetShell, "Shell", snippetPath),
			{
				Path:        snippetPath,
				Content:     renderShellSnippet(paths),
				Mode:        0o644,
				Strategy:    StrategyWrite,
				Description: "shell bootstrap snippet",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Source `%s` from your shell rc so `szr` stays the default wrapper in this repo.", relativePath(paths.RepoRoot, snippetPath)),
			fmt.Sprintf("Optionally run `%s \"$@\"` from your shell preexec hook if your shell supports command reminders.", relativePath(paths.RepoRoot, paths.HookFile)),
		},
	}
}

func renderShellSnippet(paths Paths) string {
	return fmt.Sprintf(`# szr shell bootstrap

# Load this from your shell rc for repository-local guidance.
alias szrgit='%s git'
alias szrgo='%s go'

szr_explain() {
  %s explain "$@"
}

szr_proxy() {
  %s proxy "$@"
}
`, paths.Binary, paths.Binary, paths.Binary, paths.Binary)
}

func codexReference(paths Paths) string {
	return "@" + codexDisplayPath(paths) + "\n\n" + "Use szr as the default wrapper for noisy shell commands in this repository."
}

func codexDisplayPath(paths Paths) string {
	if codexHome := os.Getenv("CODEX_HOME"); codexHome != "" {
		return filepath.ToSlash(filepath.Join(codexHome, "szr.md"))
	}
	return "~/.codex/szr.md"
}
