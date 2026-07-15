package updates

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func (s *Service) detectInstallMethod() (InstallMethod, string) {
	execPath, err := s.executable()
	if err != nil {
		return InstallMethodUnknown, ""
	}
	resolved := execPath
	if path, err := s.evalSymlinks(execPath); err == nil {
		resolved = path
	}
	resolved = filepath.Clean(resolved)

	if isBrewCellarPath(resolved) {
		return InstallMethodBrew, "brew upgrade szr"
	}
	if s.isGoInstallPath(resolved) {
		return InstallMethodGo, "go install " + goInstallRef
	}
	return InstallMethodUnknown, ""
}

func isBrewCellarPath(resolved string) bool {
	cellarFragment := string(filepath.Separator) + "Cellar" + string(filepath.Separator) + "szr" + string(filepath.Separator)
	return strings.Contains(resolved, cellarFragment)
}

func (s *Service) isGoInstallPath(resolved string) bool {
	binName := "szr"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	if gobin := strings.TrimSpace(s.getenv("GOBIN")); gobin != "" && samePath(resolved, filepath.Join(gobin, binName)) {
		return true
	}

	gopath := strings.TrimSpace(s.getenv("GOPATH"))
	if gopath == "" {
		if homeDir, err := s.userHomeDir(); err == nil {
			gopath = filepath.Join(homeDir, "go")
		}
	}
	return gopath != "" && samePath(resolved, filepath.Join(gopath, "bin", binName))
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func compareVersions(current, latest string) int {
	currentParts, currentOK := parseVersion(current)
	latestParts, latestOK := parseVersion(latest)
	if !currentOK || !latestOK {
		return 0
	}
	for i := 0; i < 3; i++ {
		switch {
		case currentParts[i] < latestParts[i]:
			return -1
		case currentParts[i] > latestParts[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(value string) ([3]int, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if value == "" {
		return [3]int{}, false
	}
	parts := strings.SplitN(value, "-", 2)
	fields := strings.Split(parts[0], ".")
	if len(fields) != 3 {
		return [3]int{}, false
	}
	var parsed [3]int
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return [3]int{}, false
		}
		parsed[i] = n
	}
	return parsed, true
}
