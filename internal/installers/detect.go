package installers

import (
	"errors"
	"os"
	"path/filepath"
)

func DetectPaths(repoRoot string) (Paths, error) {
	return DetectPathsWith(repoRoot, os.Stat)
}

func DetectClaudeGlobalPaths(homeDir string) (Paths, error) {
	return DetectClaudeGlobalPathsWith(homeDir, os.Stat)
}

func DetectPathsWith(repoRoot string, stat func(string) (os.FileInfo, error)) (Paths, error) {
	if repoRoot == "" {
		return Paths{}, errors.New("repo root is required")
	}

	absRoot, _ := filepath.Abs(repoRoot)
	info, err := stat(absRoot)
	if err != nil {
		return Paths{}, err
	}
	if !info.IsDir() {
		return Paths{}, errors.New("repo root must be a directory")
	}

	return Paths{
		RepoRoot:      absRoot,
		Binary:        detectBinary(absRoot, stat),
		HookDir:       filepath.Join(absRoot, ".szr", "hooks"),
		HookFile:      filepath.Join(absRoot, ".szr", "hooks", "pre-command.sh"),
		InstallDir:    filepath.Join(absRoot, ".szr", "install"),
		CursorRuleDir: filepath.Join(absRoot, ".cursor", "rules"),
		ClaudeDir:     filepath.Join(absRoot, ".claude"),
		ClaudeMDFile:  filepath.Join(absRoot, "CLAUDE.md"),
		ClaudeSZRFile: filepath.Join(absRoot, "szr.md"),
		ClaudeConfig:  filepath.Join(absRoot, ".claude", "settings.json"),
	}, nil
}

func DetectClaudeGlobalPathsWith(homeDir string, stat func(string) (os.FileInfo, error)) (Paths, error) {
	if homeDir == "" {
		return Paths{}, errors.New("home directory is required for claude install")
	}

	absHome, _ := filepath.Abs(homeDir)
	info, err := stat(absHome)
	if err != nil {
		return Paths{}, err
	}
	if !info.IsDir() {
		return Paths{}, errors.New("home directory must be a directory")
	}

	claudeDir := filepath.Join(absHome, ".claude")
	codexDir := resolveCodexDir(absHome)
	cursorDir := filepath.Join(absHome, ".cursor")
	geminiDir := filepath.Join(absHome, ".gemini")
	return Paths{
		RepoRoot:       absHome,
		Binary:         "szr",
		HookDir:        filepath.Join(claudeDir, "hooks"),
		HookFile:       filepath.Join(claudeDir, "hooks", "szr-rewrite.sh"),
		InstallDir:     filepath.Join(claudeDir, ".szr", "install"),
		CursorRuleDir:  filepath.Join(absHome, ".cursor", "rules"),
		ClaudeDir:      claudeDir,
		ClaudeMDFile:   filepath.Join(claudeDir, "CLAUDE.md"),
		ClaudeSZRFile:  filepath.Join(claudeDir, "szr.md"),
		ClaudeConfig:   filepath.Join(claudeDir, "settings.json"),
		CodexDir:       codexDir,
		CodexSZRFile:   filepath.Join(codexDir, "szr.md"),
		CursorDir:      cursorDir,
		CursorConfig:   filepath.Join(cursorDir, "hooks.json"),
		CursorHookDir:  filepath.Join(cursorDir, "hooks"),
		CursorHookFile: filepath.Join(cursorDir, "hooks", "szr-rewrite.sh"),
		GeminiDir:      geminiDir,
		GeminiConfig:   filepath.Join(geminiDir, "settings.json"),
		GeminiHookDir:  filepath.Join(geminiDir, "hooks"),
		GeminiHookFile: filepath.Join(geminiDir, "hooks", "szr-rewrite.sh"),
		Global:         true,
	}, nil
}

func resolveCodexDir(homeDir string) string {
	if codeXHome := os.Getenv("CODEX_HOME"); codeXHome != "" {
		if abs, err := filepath.Abs(codeXHome); err == nil {
			return abs
		}
		return codeXHome
	}
	return filepath.Join(homeDir, ".codex")
}

func detectBinary(repoRoot string, stat func(string) (os.FileInfo, error)) string {
	candidates := []struct {
		path string
		cmd  string
	}{
		{path: filepath.Join(repoRoot, "bin", "szr"), cmd: "./bin/szr"},
		{path: filepath.Join(repoRoot, "szr"), cmd: "./szr"},
	}
	for _, candidate := range candidates {
		info, err := stat(candidate.path)
		if err == nil && !info.IsDir() {
			return candidate.cmd
		}
	}

	if exists(filepath.Join(repoRoot, "go.mod"), stat) && exists(filepath.Join(repoRoot, "cmd", "szr", "main.go"), stat) {
		return "go run ./cmd/szr --"
	}

	return "szr"
}

func exists(path string, stat func(string) (os.FileInfo, error)) bool {
	info, err := stat(path)
	return err == nil && !info.IsDir()
}
