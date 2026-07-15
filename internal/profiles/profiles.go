package profiles

import (
	"time"

	"github.com/devr-tools/szr/internal/engine"
	buildprofiles "github.com/devr-tools/szr/internal/profiles/build"
	cloudlistprofiles "github.com/devr-tools/szr/internal/profiles/cloudlist"
	cloudlogsprofiles "github.com/devr-tools/szr/internal/profiles/cloudlogs"
	containerprofiles "github.com/devr-tools/szr/internal/profiles/container"
	cppprofiles "github.com/devr-tools/szr/internal/profiles/cpp"
	dotnetprofiles "github.com/devr-tools/szr/internal/profiles/dotnet"
	envdumpprofiles "github.com/devr-tools/szr/internal/profiles/envdump"
	gitprofiles "github.com/devr-tools/szr/internal/profiles/git"
	githubprofiles "github.com/devr-tools/szr/internal/profiles/github"
	gitlabprofiles "github.com/devr-tools/szr/internal/profiles/gitlab"
	httpapiprofiles "github.com/devr-tools/szr/internal/profiles/httpapi"
	javascriptprofiles "github.com/devr-tools/szr/internal/profiles/javascript"
	jsonqueryprofiles "github.com/devr-tools/szr/internal/profiles/jsonquery"
	kubernetesprofiles "github.com/devr-tools/szr/internal/profiles/kubernetes"
	patchprofiles "github.com/devr-tools/szr/internal/profiles/patch"
	phpprofiles "github.com/devr-tools/szr/internal/profiles/php"
	pythonprofiles "github.com/devr-tools/szr/internal/profiles/python"
	rustprofiles "github.com/devr-tools/szr/internal/profiles/rust"
	searchprofiles "github.com/devr-tools/szr/internal/profiles/search"
	sqlqueryprofiles "github.com/devr-tools/szr/internal/profiles/sqlquery"
	tabularprofiles "github.com/devr-tools/szr/internal/profiles/tabular"
)

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
	list = append(list, gitprofiles.Profiles(maxLines)...)
	list = append(list, buildprofiles.Profiles(maxLines)...)
	list = append(list, cppprofiles.Profiles(maxLines)...)
	list = append(list, dotnetprofiles.Profiles(maxLines)...)
	list = append(list, patchprofiles.Profiles(maxLines)...)
	list = append(list, rustprofiles.Profiles(maxLines)...)
	list = append(list, pythonprofiles.Profiles(maxLines)...)
	list = append(list, containerprofiles.Profiles(maxLines)...)
	list = append(list, cloudlistprofiles.Profiles(maxLines)...)
	list = append(list, cloudlogsprofiles.Profiles(maxLines)...)
	list = append(list, envdumpprofiles.Profiles(maxLines)...)
	list = append(list, kubernetesprofiles.Profiles(maxLines)...)
	list = append(list, githubprofiles.Profiles(maxLines)...)
	list = append(list, gitlabprofiles.Profiles(maxLines)...)
	list = append(list, httpapiprofiles.Profiles(maxLines)...)
	list = append(list, javascriptprofiles.Profiles(maxLines)...)
	list = append(list, jsonqueryprofiles.Profiles(maxLines)...)
	list = append(list, phpprofiles.Profiles(maxLines)...)
	list = append(list, searchprofiles.Profiles(maxLines, 4)...)
	list = append(list, sqlqueryprofiles.Profiles(maxLines)...)
	list = append(list, tabularprofiles.Profiles(maxLines)...)
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
