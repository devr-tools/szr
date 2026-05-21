package installers

import (
	"fmt"
	"path/filepath"
)

func renderCodex(paths Paths) Plan {
	instructionPath := filepath.Join(paths.RepoRoot, "AGENTS.md")
	return Plan{
		Target: TargetCodex,
		Title:  "Codex installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetCodex, "Codex", instructionPath),
			{
				Path:        instructionPath,
				Content:     "## szr for Codex\n\n" + renderInstructionBody(paths),
				Mode:        0o644,
				Strategy:    StrategyMerge,
				Marker:      "szr-codex",
				Description: "Codex repo instructions",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Keep `%s` committed so Codex sees the repository guidance.", relativePath(paths.RepoRoot, instructionPath)),
			fmt.Sprintf("Wire `%s \"$@\"` into Codex's pre-command hook if you want runtime reminders.", relativePath(paths.RepoRoot, paths.HookFile)),
		},
	}
}

func renderClaude(paths Paths) Plan {
	instructionPath := filepath.Join(paths.RepoRoot, "CLAUDE.md")
	return Plan{
		Target: TargetClaude,
		Title:  "Claude Code installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetClaude, "Claude Code", instructionPath),
			{
				Path:        instructionPath,
				Content:     "## szr for Claude Code\n\n" + renderInstructionBody(paths),
				Mode:        0o644,
				Strategy:    StrategyMerge,
				Marker:      "szr-claude-code",
				Description: "Claude Code repo instructions",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Keep `%s` committed so Claude Code inherits the repository guidance.", relativePath(paths.RepoRoot, instructionPath)),
			fmt.Sprintf("Wire `%s \"$@\"` into Claude Code's pre-command hook if you want runtime reminders.", relativePath(paths.RepoRoot, paths.HookFile)),
		},
	}
}

func renderCursor(paths Paths) Plan {
	rulePath := filepath.Join(paths.CursorRuleDir, "szr.mdc")
	return Plan{
		Target: TargetCursor,
		Title:  "Cursor installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetCursor, "Cursor", rulePath),
			{
				Path:        rulePath,
				Content:     renderCursorRule(paths),
				Mode:        0o644,
				Strategy:    StrategyWrite,
				Description: "Cursor rule",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Commit `%s` so Cursor auto-applies the repository rule.", relativePath(paths.RepoRoot, rulePath)),
			fmt.Sprintf("Add `%s \"$@\"` as a pre-command reminder in Cursor if you want shell hints.", relativePath(paths.RepoRoot, paths.HookFile)),
		},
	}
}

func renderGemini(paths Paths) Plan {
	instructionPath := filepath.Join(paths.RepoRoot, "GEMINI.md")
	return Plan{
		Target: TargetGemini,
		Title:  "Gemini installer",
		Paths:  paths,
		Files: []File{
			sharedHookFile(paths),
			sharedInstallDoc(paths, TargetGemini, "Gemini", instructionPath),
			{
				Path:        instructionPath,
				Content:     "## szr for Gemini\n\n" + renderInstructionBody(paths),
				Mode:        0o644,
				Strategy:    StrategyMerge,
				Marker:      "szr-gemini",
				Description: "Gemini repo instructions",
			},
		},
		ManualSteps: []string{
			fmt.Sprintf("Keep `%s` committed so Gemini sees the repository guidance.", relativePath(paths.RepoRoot, instructionPath)),
			fmt.Sprintf("Wire `%s \"$@\"` into Gemini's pre-command hook if you want runtime reminders.", relativePath(paths.RepoRoot, paths.HookFile)),
		},
	}
}

func renderCursorRule(paths Paths) string {
	return fmt.Sprintf(`---
description: Use szr for noisy shell commands in this repository.
globs:
alwaysApply: true
---

# szr workflow

%s
`, renderInstructionBody(paths))
}
