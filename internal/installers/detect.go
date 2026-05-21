package installers

import (
	"errors"
	"os"
	"path/filepath"
)

func DetectPaths(repoRoot string) (Paths, error) {
	return DetectPathsWith(repoRoot, os.Stat)
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
	}, nil
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
