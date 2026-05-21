package profiles

import "szr/internal/engine"

func Builtins(maxLines int) []engine.Profile {
	list := coreProfiles(maxLines)
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
