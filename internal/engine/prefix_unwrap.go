package engine

import (
	"path/filepath"
	"strings"
)

// stripTransparentPrefix peels well-known transparent wrapper prefixes —
// `env` (assignments and `-u` unsets only), `command`, `nice`, bare `time`,
// and leading VAR=value assignments — off an argv so the inner command can
// be classified and matched against its own profile. Prefixes compose:
// `env FOO=1 nice go test` strips down to `go test`. A wrapper that consumes
// the whole argv (bare `env`, `env FOO=bar` with no trailing command) is the
// command itself, not a prefix, and stays untouched.
func stripTransparentPrefix(args []string) (prefix, inner []string, ok bool) {
	rest := args
	consumed := 0
	for len(rest) > 0 {
		width, strippable := transparentWrapperWidth(rest)
		if !strippable || width >= len(rest) {
			break
		}
		rest = rest[width:]
		consumed += width
	}
	if consumed == 0 || len(rest) == 0 {
		return nil, nil, false
	}
	return append([]string(nil), args[:consumed]...), append([]string(nil), rest...), true
}

// transparentWrapperWidth returns the number of leading words forming one
// transparent wrapper unit, or ok=false when the head is not a wrapper we
// can strip cleanly (unknown command, or a wrapper flag we do not model).
func transparentWrapperWidth(args []string) (int, bool) {
	if isEnvAssignment(args[0]) {
		i := 1
		for i < len(args) && isEnvAssignment(args[i]) {
			i++
		}
		return i, true
	}
	switch filepath.Base(args[0]) {
	case "env":
		return envPrefixWidth(args)
	case "command":
		return commandPrefixWidth(args)
	case "nice":
		return nicePrefixWidth(args)
	case "time":
		return timePrefixWidth(args)
	default:
		return 0, false
	}
}

// envPrefixWidth models `env [KEY=VALUE ...] [-u NAME ...] cmd ...`. Any
// other env flag (-i, -S, --split-string, ...) disqualifies the strip: the
// remainder would not be a plain inner command.
func envPrefixWidth(args []string) (int, bool) {
	i := 1
	for i < len(args) {
		switch {
		case isEnvAssignment(args[i]):
			i++
		case args[i] == "-u":
			if i+1 >= len(args) {
				return 0, false
			}
			i += 2
		case hasShortAttachedValue(args[i], "-u"):
			i++
		case strings.HasPrefix(args[i], "-"):
			return 0, false
		default:
			return i, true
		}
	}
	return i, true
}

// commandPrefixWidth models the POSIX `command [-p] cmd ...` builtin form.
// `-v`/`-V` change the output entirely and disqualify the strip.
func commandPrefixWidth(args []string) (int, bool) {
	i := 1
	for i < len(args) && args[i] == "-p" {
		i++
	}
	if i < len(args) && strings.HasPrefix(args[i], "-") {
		return 0, false
	}
	return i, true
}

// nicePrefixWidth models `nice cmd ...` and `nice -n <N> cmd ...`.
func nicePrefixWidth(args []string) (int, bool) {
	i := 1
	if i < len(args) && args[i] == "-n" {
		if i+1 >= len(args) {
			return 0, false
		}
		i += 2
	}
	if i < len(args) && strings.HasPrefix(args[i], "-") {
		return 0, false
	}
	return i, true
}

// timePrefixWidth models the bare `time cmd ...` prefix form only; any flag
// after `time` disqualifies the strip.
func timePrefixWidth(args []string) (int, bool) {
	if len(args) > 1 && strings.HasPrefix(args[1], "-") {
		return 0, false
	}
	return 1, true
}
