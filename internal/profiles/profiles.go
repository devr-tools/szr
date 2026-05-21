package profiles

import (
	"time"

	"szr/internal/engine"
)

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
	list = append(list, rustProfiles(maxLines)...)
	list = append(list, pythonProfiles(maxLines)...)
	list = append(list, containerProfiles(maxLines)...)
	list = append(list, jsProfiles(maxLines)...)
	return list
}

func parseStdout(exec engine.Execution) int {
	return len(exec.Stdout)
}

func parseCombined(exec engine.Execution) int {
	return len(exec.Stdout) + len(exec.Stderr)
}

func parseStderrFirst(exec engine.Execution) int {
	if exec.Stderr == "" {
		return len(exec.Stdout)
	}
	return len(exec.Stderr) + len(exec.Stdout)
}

func outputBudget(lines int) engine.OutputBudget {
	if lines <= 0 {
		lines = 12
	}
	return engine.OutputBudget{
		MaxLines:  lines,
		MaxBytes:  lines * 160,
		MaxTokens: lines * 32,
	}
}

func latencyBudget(ms int) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}
