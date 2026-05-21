package engine

import (
	"bytes"
	"context"
	"os/exec"
)

func runCommand(ctx context.Context, args []string, cwd string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = cwd

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return stdout.String(), stderr.String(), 0, nil
	}

	exitCode := 1
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
		return stdout.String(), stderr.String(), exitCode, nil
	}
	return stdout.String(), stderr.String(), exitCode, err
}
