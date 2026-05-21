package profiles

import (
	"time"

	"szr/internal/engine"
	containerprofiles "szr/internal/profiles/container"
	githubprofiles "szr/internal/profiles/github"
	javascriptprofiles "szr/internal/profiles/javascript"
	kubernetesprofiles "szr/internal/profiles/kubernetes"
	pythonprofiles "szr/internal/profiles/python"
	rustprofiles "szr/internal/profiles/rust"
)

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
	list = append(list, rustprofiles.Profiles(maxLines)...)
	list = append(list, pythonprofiles.Profiles(maxLines)...)
	list = append(list, containerprofiles.Profiles(maxLines)...)
	list = append(list, kubernetesprofiles.Profiles(maxLines)...)
	list = append(list, githubprofiles.Profiles(maxLines)...)
	list = append(list, javascriptprofiles.Profiles(maxLines)...)
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
