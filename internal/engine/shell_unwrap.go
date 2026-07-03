package engine

import (
	"path/filepath"
	"strings"
)

// ShellWrap records how a shell `-c` wrapper invocation was unwrapped for
// classification and matching. Execution always runs the ORIGINAL wrapper
// argv: profile Prepare rewrites are translated back into the wrapper's
// command string only when the final segment re-quotes losslessly
// (LiteralSafe), and are suppressed otherwise.
type ShellWrap struct {
	// Original is the exact argv the user invoked, wrapper included.
	Original []string
	// CommandArg is the index in Original of the `-c` command string.
	CommandArg int
	// Prefix is the verbatim slice of the command string preceding the
	// final segment (setup commands plus their separator), kept byte-exact
	// when the command string is rebuilt.
	Prefix string
	// LiteralSafe reports that every word of the final segment parsed
	// without quoting-sensitive constructs (expansions, globs, tildes), so
	// re-quoting the words individually preserves the shell's semantics.
	LiteralSafe bool
}

// execCommand returns the argv to execute for a prepared inner command.
// When Prepare rewrote the inner argv and the final segment is literal-safe,
// the wrapper's command string is rebuilt around the rewritten words.
// Otherwise the original wrapper argv runs untouched: suppressing the
// Prepare rewrite is strictly safer than splicing flags into a command
// string whose words we cannot re-quote losslessly (expansions and globs
// would turn literal), and profile renders still work on the raw output.
func (w *ShellWrap) execCommand(matchedInner, preparedInner []string) []string {
	if !w.LiteralSafe || sameStrings(matchedInner, preparedInner) {
		return append([]string(nil), w.Original...)
	}
	out := append([]string(nil), w.Original...)
	out[w.CommandArg] = w.Prefix + joinShellWords(preparedInner)
	return out
}

// unwrapShellInvocation rewrites the effective invocation's Command and
// Display to the wrapper's inner command for classification and matching.
// The recorded ShellWrap keeps the original argv for execution; history and
// user-facing display always come from the caller's original invocation.
func unwrapShellInvocation(inv Invocation) Invocation {
	if wrap, inner, ok := unwrapShellWrapper(inv.Command); ok {
		inv.ShellWrap = wrap
		inv.Command = inner
	}
	if inner, ok := unwrapShellCommandArgs(inv.Display); ok {
		inv.Display = inner
	}
	return inv
}

var shellWrapperNames = map[string]bool{
	"sh":   true,
	"bash": true,
	"zsh":  true,
	"dash": true,
	"ksh":  true,
}

// unwrapShellWrapper unwraps `sh|bash|zsh|dash|ksh [-flags] -c <string>`
// wrappers whose command string is (a) a single simple command, or (b) such
// a command preceded by setup-only segments (source/cd/export/assignments)
// joined with `&&` or `;`. Anything else — pipes, redirection, substitution,
// `||`, multiple real commands, nested `-c` wrappers — stays wrapped; the
// pipeline policy in `szr rewrite` owns that space.
func unwrapShellWrapper(args []string) (*ShellWrap, []string, bool) {
	cIdx := shellWrapperCommandIndex(args)
	if cIdx < 0 {
		return nil, nil, false
	}
	last, ok := finalCommandSegment(args[cIdx])
	if !ok {
		return nil, nil, false
	}
	wrap := &ShellWrap{
		Original:    append([]string(nil), args...),
		CommandArg:  cIdx,
		Prefix:      args[cIdx][:last.start],
		LiteralSafe: last.safe,
	}
	return wrap, append([]string(nil), last.words...), true
}

// finalCommandSegment parses a `-c` command string and returns its final
// segment when every earlier segment is setup-only and the final segment is
// a simple, non-empty command. Nested `-c` wrappers never qualify: they
// cannot be rebuilt safely inside the outer command string.
func finalCommandSegment(command string) (shellSegment, bool) {
	segments, ok := parseShellSegments(command)
	if !ok || len(segments) == 0 {
		return shellSegment{}, false
	}
	for _, segment := range segments[:len(segments)-1] {
		if !isShellSetupSegment(segment.words) {
			return shellSegment{}, false
		}
	}
	last := segments[len(segments)-1]
	if len(last.words) == 0 || shellWrapperCommandIndex(last.words) >= 0 {
		return shellSegment{}, false
	}
	return last, true
}

func unwrapShellCommandArgs(args []string) ([]string, bool) {
	_, inner, ok := unwrapShellWrapper(args)
	return inner, ok
}

// shellWrapperCommandIndex returns the index of the `-c` command string in
// a shell wrapper argv, or -1. Only bare short-flag clusters (-c, -l, -lc,
// -l -c, -lic, ...) are tolerated before the cluster containing `c`;
// long options, option values, and `--` disqualify the wrapper. Positional
// arguments after the command string ($0/$@) are allowed and ignored.
func shellWrapperCommandIndex(args []string) int {
	if len(args) < 3 || !shellWrapperNames[filepath.Base(args[0])] {
		return -1
	}
	for i := 1; i < len(args); i++ {
		cluster, ok := shellFlagCluster(args[i])
		if !ok {
			return -1
		}
		if strings.ContainsRune(cluster, 'c') {
			if i+1 >= len(args) {
				return -1
			}
			return i + 1
		}
	}
	return -1
}

func shellFlagCluster(arg string) (string, bool) {
	if len(arg) < 2 || arg[0] != '-' {
		return "", false
	}
	for _, r := range arg[1:] {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return "", false
		}
	}
	return arg[1:], true
}

// isShellSetupSegment reports whether a segment only prepares the
// environment for a later command: source/dot includes, cd, export, set,
// unset, and eval or bare segments consisting purely of VAR=value
// assignments.
func isShellSetupSegment(words []string) bool {
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "source", ".", "cd", "export", "set", "unset":
		return true
	case "eval":
		return allEnvAssignments(words[1:])
	default:
		return allEnvAssignments(words)
	}
}

func allEnvAssignments(words []string) bool {
	if len(words) == 0 {
		return false
	}
	for _, word := range words {
		if !isEnvAssignment(word) {
			return false
		}
	}
	return true
}

// joinShellWords rebuilds a command string from argv words, single-quoting
// any word containing characters outside a conservative literal set.
func joinShellWords(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		quoted = append(quoted, quoteShellWord(word))
	}
	return strings.Join(quoted, " ")
}

func quoteShellWord(word string) string {
	if word != "" && !strings.ContainsFunc(word, needsShellQuoting) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func needsShellQuoting(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	switch r {
	case '_', '-', '.', '/', ':', '=', ',', '@', '+', '%':
		return false
	default:
		return true
	}
}
