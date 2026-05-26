package search

import "github.com/devr-tools/szr/internal/profilekit"

func isPlainFindCommand(args []string) bool {
	if len(args) == 0 || args[0] != "find" {
		return false
	}
	if profilekit.ContainsAny(args[1:], "-exec", "-execdir", "-delete", "-printf", "-print0", "-quit", "-prune") {
		return false
	}
	return true
}

func prepareFind(command []string) []string {
	if len(command) < 2 || !shouldInjectDefaultFindExcludes(command[1:]) {
		return command
	}

	out := append([]string{}, command[:2]...)
	for _, dir := range defaultSearchExcludeDirs() {
		out = append(out, "-not", "-path", "*/"+dir+"/*")
	}
	return append(out, command[2:]...)
}

func shouldInjectDefaultFindExcludes(args []string) bool {
	if len(args) == 0 || args[0] != "." {
		return false
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-path" || arg == "-prune" || arg == "-not" || arg == "!" || arg == "-o" || arg == "-or" {
			return false
		}
	}
	return true
}
