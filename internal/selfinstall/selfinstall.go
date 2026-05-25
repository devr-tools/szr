package selfinstall

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Plan struct {
	ExecutablePath string
	InstallDir     string
	TargetPath     string
	ShellName      string
	ShellRCPath    string
	ShellSnippet   string
	PathContains   bool
}

type Result struct {
	Plan            Plan
	Installed       bool
	ShellConfigured bool
	ShellUpdated    bool
}

func PlanInstall(executablePath, homeDir, pathEnv, shell string, create bool) (Plan, error) {
	return PlanInstallWith(executablePath, homeDir, pathEnv, shell, create, os.Stat, os.MkdirAll)
}

func PlanInstallWith(
	executablePath, homeDir, pathEnv, shell string,
	create bool,
	stat func(string) (os.FileInfo, error),
	mkdirAll func(string, os.FileMode) error,
) (Plan, error) {
	if strings.TrimSpace(homeDir) == "" {
		return Plan{}, errors.New("home directory is required")
	}
	if strings.TrimSpace(executablePath) == "" {
		return Plan{}, errors.New("executable path is required")
	}

	installDir, err := chooseInstallDir(homeDir, create, stat, mkdirAll)
	if err != nil {
		return Plan{}, err
	}
	shellName, shellRCPath, shellSnippet := shellPlan(homeDir, shell, installDir)
	targetPath := filepath.Join(installDir, binaryName())
	return Plan{
		ExecutablePath: executablePath,
		InstallDir:     installDir,
		TargetPath:     targetPath,
		ShellName:      shellName,
		ShellRCPath:    shellRCPath,
		ShellSnippet:   shellSnippet,
		PathContains:   pathContainsDir(pathEnv, installDir),
	}, nil
}

func WithInstallDir(plan Plan, homeDir, pathEnv, shell, installDir string) Plan {
	shellName, shellRCPath, shellSnippet := shellPlan(homeDir, shell, installDir)
	plan.InstallDir = installDir
	plan.TargetPath = filepath.Join(installDir, binaryName())
	plan.ShellName = shellName
	plan.ShellRCPath = shellRCPath
	plan.ShellSnippet = shellSnippet
	plan.PathContains = pathContainsDir(pathEnv, installDir)
	return plan
}

func Install(plan Plan, updateShell bool) (Result, error) {
	return InstallWith(plan, updateShell, os.ReadFile, os.WriteFile, os.MkdirAll)
}

func InstallWith(
	plan Plan,
	updateShell bool,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	mkdirAll func(string, os.FileMode) error,
) (Result, error) {
	if err := validateInstallPlan(plan); err != nil {
		return Result{}, err
	}

	result := Result{Plan: plan}
	installed, err := installExecutable(plan, readFile, writeFile, mkdirAll)
	if err != nil {
		return Result{}, err
	}
	result.Installed = installed

	shellResult, err := configureShellInstall(plan, updateShell, readFile, writeFile, mkdirAll)
	if err != nil {
		return Result{}, err
	}
	result.ShellConfigured = shellResult.ShellConfigured
	result.ShellUpdated = shellResult.ShellUpdated
	return result, nil
}

func validateInstallPlan(plan Plan) error {
	if strings.TrimSpace(plan.ExecutablePath) == "" || strings.TrimSpace(plan.TargetPath) == "" {
		return errors.New("install plan is incomplete")
	}
	return nil
}

func installExecutable(
	plan Plan,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	mkdirAll func(string, os.FileMode) error,
) (bool, error) {
	if filepath.Clean(plan.ExecutablePath) != filepath.Clean(plan.TargetPath) {
		data, err := readFile(plan.ExecutablePath)
		if err != nil {
			return false, err
		}
		if err := mkdirAll(filepath.Dir(plan.TargetPath), 0o755); err != nil {
			return false, err
		}
		if err := writeFile(plan.TargetPath, data, 0o755); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func configureShellInstall(
	plan Plan,
	updateShell bool,
	readFile func(string) ([]byte, error),
	writeFile func(string, []byte, os.FileMode) error,
	mkdirAll func(string, os.FileMode) error,
) (Result, error) {
	result := Result{}
	if plan.ShellSnippet == "" || plan.ShellRCPath == "" {
		return result, nil
	}

	existing, err := readFile(plan.ShellRCPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	if strings.Contains(string(existing), plan.ShellSnippet) {
		result.ShellConfigured = true
		return result, nil
	}
	if !updateShell {
		return result, nil
	}

	if err := mkdirAll(filepath.Dir(plan.ShellRCPath), 0o755); err != nil {
		return Result{}, err
	}
	content := strings.TrimRight(string(existing), "\n")
	if content != "" {
		content += "\n\n"
	}
	content += plan.ShellSnippet + "\n"
	if err := writeFile(plan.ShellRCPath, []byte(content), 0o644); err != nil {
		return Result{}, err
	}
	result.ShellConfigured = true
	result.ShellUpdated = true
	return result, nil
}

func chooseInstallDir(
	homeDir string,
	create bool,
	stat func(string) (os.FileInfo, error),
	mkdirAll func(string, os.FileMode) error,
) (string, error) {
	candidates := []string{
		filepath.Join(homeDir, ".local", "bin"),
		filepath.Join(homeDir, "bin"),
	}
	var firstMissing string
	for _, candidate := range candidates {
		info, err := stat(candidate)
		switch {
		case err == nil && info.IsDir():
			return candidate, nil
		case err == nil && !info.IsDir():
			continue
		case errors.Is(err, os.ErrNotExist):
			if firstMissing == "" {
				firstMissing = candidate
			}
			if create {
				if mkdirErr := mkdirAll(candidate, 0o755); mkdirErr == nil {
					return candidate, nil
				}
			}
		}
	}
	if !create && firstMissing != "" {
		return firstMissing, nil
	}
	return "", fmt.Errorf("failed to resolve install directory under %s", homeDir)
}

func pathContainsDir(pathEnv, dir string) bool {
	target := filepath.Clean(dir)
	for _, entry := range filepath.SplitList(pathEnv) {
		if entry == "" {
			continue
		}
		if filepath.Clean(entry) == target {
			return true
		}
	}
	return false
}

func shellPlan(homeDir, shell, installDir string) (string, string, string) {
	name := filepath.Base(strings.TrimSpace(shell))
	if name == "" {
		name = "sh"
	}
	pathExpr := pathExpression(homeDir, installDir)
	switch name {
	case "zsh":
		return name, filepath.Join(homeDir, ".zshrc"), fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr)
	case "bash":
		return name, filepath.Join(homeDir, ".bashrc"), fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr)
	case "fish":
		return name, filepath.Join(homeDir, ".config", "fish", "config.fish"), fmt.Sprintf(`fish_add_path -p %s`, pathExpr)
	default:
		return name, filepath.Join(homeDir, ".profile"), fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr)
	}
}

func pathExpression(homeDir, installDir string) string {
	homeDir = filepath.Clean(homeDir)
	installDir = filepath.Clean(installDir)
	if rel, err := filepath.Rel(homeDir, installDir); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
		return "$HOME/" + filepath.ToSlash(rel)
	}
	if installDir == homeDir {
		return "$HOME"
	}
	return filepath.ToSlash(installDir)
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "szr.exe"
	}
	return "szr"
}
