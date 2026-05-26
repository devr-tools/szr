package profiles

import (
	"time"

	"github.com/devr-tools/szr/internal/engine"
	buildprofiles "github.com/devr-tools/szr/internal/profiles/build"
	containerprofiles "github.com/devr-tools/szr/internal/profiles/container"
	cppprofiles "github.com/devr-tools/szr/internal/profiles/cpp"
	gitprofiles "github.com/devr-tools/szr/internal/profiles/git"
	githubprofiles "github.com/devr-tools/szr/internal/profiles/github"
	javascriptprofiles "github.com/devr-tools/szr/internal/profiles/javascript"
	kubernetesprofiles "github.com/devr-tools/szr/internal/profiles/kubernetes"
	patchprofiles "github.com/devr-tools/szr/internal/profiles/patch"
	phpprofiles "github.com/devr-tools/szr/internal/profiles/php"
	pythonprofiles "github.com/devr-tools/szr/internal/profiles/python"
	rustprofiles "github.com/devr-tools/szr/internal/profiles/rust"
	searchprofiles "github.com/devr-tools/szr/internal/profiles/search"
)

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
	list = append(list, gitprofiles.Profiles(maxLines)...)
	list = append(list, buildprofiles.Profiles(maxLines)...)
	list = append(list, cppprofiles.Profiles(maxLines)...)
	list = append(list, patchprofiles.Profiles(maxLines)...)
	list = append(list, rustprofiles.Profiles(maxLines)...)
	list = append(list, pythonprofiles.Profiles(maxLines)...)
	list = append(list, containerprofiles.Profiles(maxLines)...)
	list = append(list, kubernetesprofiles.Profiles(maxLines)...)
	list = append(list, githubprofiles.Profiles(maxLines)...)
	list = append(list, javascriptprofiles.Profiles(maxLines)...)
	list = append(list, phpprofiles.Profiles(maxLines)...)
	list = append(list, searchprofiles.Profiles(maxLines, 4)...)
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
