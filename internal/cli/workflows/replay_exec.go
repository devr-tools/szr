package workflows

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/engine"
)

func captureExecution(ctx context.Context, command []string, cwd string) (engine.Execution, time.Duration, error) {
	if len(command) == 0 {
		return engine.Execution{}, 0, fmt.Errorf("missing command")
	}
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return engine.Execution{}, 0, err
		}
	}
	return engine.Execution{
		Command:  append([]string(nil), command...),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: duration,
	}, duration, nil
}

func resolveExecutableCommand(command []string) ([]string, error) {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, fmt.Errorf("missing command")
	}
	for _, arg := range command {
		if strings.TrimSpace(arg) == "" {
			return nil, fmt.Errorf("command contains an empty argument")
		}
	}
	if isShellTrampoline(command[0]) {
		return nil, fmt.Errorf("shell wrapper commands are not allowed in compare; run the target binary directly")
	}
	resolved, err := exec.LookPath(command[0])
	if err != nil {
		return nil, err
	}
	return append([]string{resolved}, command[1:]...), nil
}

func isShellTrampoline(name string) bool {
	switch strings.ToLower(filepath.Base(name)) {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "csh", "tcsh", "pwsh", "powershell", "cmd", "cmd.exe":
		return true
	default:
		return false
	}
}
