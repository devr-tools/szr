package workflows

import (
	"fmt"
	"strconv"
	"strings"
)

func parseReplayArgs(rt Runtime, args []string) (replayOptions, int) {
	opts := replayOptions{}
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			nextIndex, code := applyReplayFlag(rt, args, i, &opts)
			if code != 0 {
				return replayOptions{}, code
			}
			i = nextIndex
			continue
		}
		if code := setReplayTarget(rt, &opts, args[i]); code != 0 {
			return replayOptions{}, code
		}
	}
	if opts.target == "" {
		fmt.Fprintln(rt.Stderr, "szr: replay requires a tee id or file path")
		return replayOptions{}, 2
	}
	return opts, 0
}

func applyReplayFlag(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	switch args[index] {
	case "--json":
		opts.asJSON = true
		return index, 0
	case "--command":
		return setReplayStringOption(rt, args, index, "--command", &opts.commandText)
	case "--profile":
		return setReplayStringOption(rt, args, index, "--profile", &opts.profileName)
	case "--cwd":
		return setReplayStringOption(rt, args, index, "--cwd", &opts.overrideCwd)
	case "--exit-code":
		return setReplayExitCode(rt, args, index, opts)
	case "--max-lines":
		return setReplayMaxLines(rt, args, index, opts)
	default:
		fmt.Fprintf(rt.Stderr, "szr: unknown replay flag %s\n", args[index])
		return index, 2
	}
}

func setReplayStringOption(rt Runtime, args []string, index int, flag string, target *string) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, flag)
	if !ok {
		return index, 2
	}
	*target = value
	return index, 0
}

func setReplayExitCode(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, "--exit-code")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "szr: invalid replay exit code %q\n", value)
		return index, 2
	}
	opts.overrideExitCode = parsed
	opts.overrideExitSet = true
	return index, 0
}

func setReplayMaxLines(rt Runtime, args []string, index int, opts *replayOptions) (int, int) {
	value, ok := requireReplayValue(rt, args, &index, "--max-lines")
	if !ok {
		return index, 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		fmt.Fprintf(rt.Stderr, "szr: invalid replay max lines %q\n", value)
		return index, 2
	}
	opts.maxLines = parsed
	return index, 0
}

func setReplayTarget(rt Runtime, opts *replayOptions, target string) int {
	if opts.target != "" {
		fmt.Fprintln(rt.Stderr, "szr: replay accepts exactly one tee id or file path")
		return 2
	}
	opts.target = target
	return 0
}

func requireReplayValue(rt Runtime, args []string, index *int, flag string) (string, bool) {
	if *index+1 >= len(args) {
		fmt.Fprintf(rt.Stderr, "szr: replay requires a value after %s\n", flag)
		return "", false
	}
	*index = *index + 1
	return args[*index], true
}
