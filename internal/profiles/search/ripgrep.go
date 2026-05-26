package search

import (
	"strings"

	"github.com/devr-tools/szr/internal/profilekit"
)

func isRipgrepCommand(args []string) bool {
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	if profilekit.ContainsAny(args[1:], "--json", "--files", "--files-with-matches", "-l", "--count", "-c", "--count-matches") {
		return false
	}
	return true
}

func isRipgrepFilesCommand(args []string) bool {
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	return profilekit.ContainsAny(args[1:], "--files")
}

func prepareRipgrep(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	if !profilekit.ContainsAny(out[1:], "-n", "--line-number") {
		out = append(out[:1], append([]string{"-n"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "-H", "--with-filename") {
		out = append(out[:1], append([]string{"-H"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "--no-heading", "--heading") {
		out = append(out[:1], append([]string{"--no-heading"}, out[1:]...)...)
	}
	if !profilekit.ContainsAny(out[1:], "--color", "never", "always", "ansi", "auto") && !profilekit.ContainsPrefix(out[1:], "--color=") {
		out = append(out[:1], append([]string{"--color=never"}, out[1:]...)...)
	}
	if shouldInjectDefaultRipgrepExcludes(out[1:]) {
		injected := []string{out[0]}
		for _, glob := range defaultSearchExcludeGlobs() {
			injected = append(injected, "-g", glob)
		}
		out = append(injected, out[1:]...)
	}
	return out
}

func prepareRipgrepFiles(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	if shouldInjectDefaultRipgrepExcludes(out[1:]) {
		injected := []string{out[0]}
		for _, glob := range defaultSearchExcludeGlobs() {
			injected = append(injected, "-g", glob)
		}
		out = append(injected, out[1:]...)
	}
	return out
}

func shouldInjectDefaultRipgrepExcludes(args []string) bool {
	if len(args) == 0 {
		return false
	}
	positionals, ok := ripgrepPositionals(args)
	if !ok {
		return false
	}
	if len(positionals) <= 1 {
		return true
	}
	for _, path := range positionals[1:] {
		if path != "." && path != "./" {
			return false
		}
	}
	return true
}

func ripgrepPositionals(args []string) ([]string, bool) {
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return append(positionals, args[i+1:]...), true
		case consumesRipgrepValue(arg):
			i++
		case isRipgrepExcludeOverride(arg), isRipgrepIgnoreBypass(arg):
			return nil, false
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			positionals = append(positionals, arg)
		}
	}
	return positionals, true
}

func isRipgrepExcludeOverride(arg string) bool {
	return strings.HasPrefix(arg, "-g") || strings.HasPrefix(arg, "--glob") || strings.HasPrefix(arg, "--iglob")
}

func isRipgrepIgnoreBypass(arg string) bool {
	switch arg {
	case "--hidden", "--no-ignore", "--no-ignore-vcs", "--no-ignore-parent", "-u", "-uu", "-uuu":
		return true
	default:
		return false
	}
}

func consumesRipgrepValue(arg string) bool {
	switch arg {
	case "-e", "-f", "-g", "--glob", "--iglob", "--ignore-file", "--max-filesize", "--type", "-t", "--type-not", "-T":
		return true
	default:
		return false
	}
}
