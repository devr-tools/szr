package search

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/profilekit"
)

func isRipgrepCommand(args []string) bool {
	args = engine.CanonicalArgsForClassification(args)
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	if profilekit.ContainsAny(args[1:], "--json", "--files", "--files-with-matches", "-l", "--count", "-c", "--count-matches") {
		return false
	}
	return true
}

func isRecursiveGrepCommand(args []string) bool {
	args = engine.CanonicalArgsForClassification(args)
	if len(args) == 0 || args[0] != "grep" {
		return false
	}
	if !containsRecursiveGrepFlag(args[1:]) {
		return false
	}
	return !hasGrepOutputShapeOverride(args[1:])
}

// isStdinGrepCommand reports whether the invocation is a non-recursive grep
// with a pattern but no path operands, meaning grep reads from stdin (the
// typical pipeline filter shape, e.g. `... | grep -n '^#'`).
func isStdinGrepCommand(args []string) bool {
	args = engine.CanonicalArgsForClassification(args)
	if len(args) == 0 || args[0] != "grep" {
		return false
	}
	if containsRecursiveGrepFlag(args[1:]) {
		return false
	}
	if hasGrepOutputShapeOverride(args[1:]) {
		return false
	}
	return grepReadsStdin(args[1:])
}

func hasGrepOutputShapeOverride(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-h", "-l", "-L", "-c", "-q", "-o", "-v", "--no-filename", "--files-with-matches", "--files-without-match", "--count", "--quiet", "--only-matching", "--invert-match":
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "h") {
			return true
		}
	}
	return false
}

// grepReadsStdin reports whether grep args (head excluded) leave grep without
// any path operands, so it reads from stdin. Callers must first rule out
// recursive flags: recursive grep without a path defaults to "." instead of
// stdin. Parsing is conservative: an argument that cannot be confidently
// classified counts as a path operand, which keeps the invocation out of
// stdin mode.
func grepReadsStdin(args []string) bool {
	positionals, patternFromFlag := grepOperands(args)
	if patternFromFlag {
		return positionals == 0
	}
	return positionals == 1
}

// grepOperands counts non-flag operands and reports whether the pattern is
// supplied via -e/-f style flags instead of the first operand.
func grepOperands(args []string) (positionals int, patternFromFlag bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return positionals + len(args) - i - 1, patternFromFlag
		case isGrepPatternFlag(arg):
			patternFromFlag = true
			if !strings.Contains(arg, "=") {
				i++
			}
		case consumesGrepValue(arg):
			i++
		case strings.HasPrefix(arg, "-") && arg != "-":
			continue
		default:
			positionals++
		}
	}
	return positionals, patternFromFlag
}

func isGrepPatternFlag(arg string) bool {
	switch arg {
	case "-e", "-f", "--regexp", "--file":
		return true
	default:
		return strings.HasPrefix(arg, "--regexp=") || strings.HasPrefix(arg, "--file=")
	}
}

// isStdinGrepInvocation mirrors the stdin-mode matcher for render-time
// dispatch, preferring the display args the matcher classified.
func isStdinGrepInvocation(inv engine.Invocation) bool {
	args := inv.Display
	if len(args) == 0 {
		args = inv.Command
	}
	return isStdinGrepCommand(args)
}

func consumesGrepValue(arg string) bool {
	switch arg {
	case "-m", "-A", "-B", "-C", "-d", "-D",
		"--max-count", "--after-context", "--before-context", "--context",
		"--label", "--binary-files", "--devices", "--directories",
		"--include", "--exclude", "--exclude-dir", "--exclude-from":
		return true
	default:
		return false
	}
}

func isRipgrepFilesCommand(args []string) bool {
	args = engine.CanonicalArgsForClassification(args)
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	return profilekit.ContainsAny(args[1:], "--files")
}

func isRipgrepFilesWithMatchesCommand(args []string) bool {
	args = engine.CanonicalArgsForClassification(args)
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	return profilekit.ContainsAny(args[1:], "--files-with-matches", "-l")
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

func prepareGrep(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	// A grep reading stdin must keep its output byte-compatible with the
	// user's invocation: injecting -n or -H would prefix every line with
	// line numbers or "(standard input)", breaking pipeline semantics.
	// Leave stdin filters entirely untouched.
	if !containsRecursiveGrepFlag(out[1:]) && grepReadsStdin(out[1:]) {
		return out
	}
	if !containsGrepLineNumberFlag(out[1:]) {
		out = append(out[:1], append([]string{"-n"}, out[1:]...)...)
	}
	if !containsGrepFilenameFlag(out[1:]) {
		out = append(out[:1], append([]string{"-H"}, out[1:]...)...)
	}
	if !containsGrepColorFlag(out[1:]) {
		out = append(out[:1], append([]string{"--color=never"}, out[1:]...)...)
	}
	if shouldInjectDefaultGrepExcludes(out[1:]) {
		injected := []string{out[0]}
		for _, dir := range defaultSearchExcludeDirs() {
			injected = append(injected, "--exclude-dir="+dir)
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

func containsRecursiveGrepFlag(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-r", "-R", "--recursive", "--dereference-recursive":
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "r") {
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "R") {
			return true
		}
	}
	return false
}

func containsGrepLineNumberFlag(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-n", "--line-number":
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "n") {
			return true
		}
	}
	return false
}

func containsGrepFilenameFlag(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-H", "--with-filename":
			return true
		case "-h", "--no-filename":
			return false
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "H") {
			return true
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "h") {
			return false
		}
	}
	return false
}

func containsGrepColorFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--color" || strings.HasPrefix(arg, "--color=") {
			return true
		}
	}
	return false
}

func shouldInjectDefaultGrepExcludes(args []string) bool {
	if len(args) == 0 {
		return false
	}
	hasRecursive := containsRecursiveGrepFlag(args)
	positionals := 0
	for _, arg := range args {
		switch {
		case arg == "--exclude-dir" || strings.HasPrefix(arg, "--exclude-dir="):
			return false
		case arg == "--":
			return false
		case strings.HasPrefix(arg, "-"):
			continue
		default:
			positionals++
			if positionals > 1 && arg != "." && arg != "./" {
				return false
			}
		}
	}
	return hasRecursive
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
