package search

import (
	"strings"

	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/filters"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int, maxGroups int) []engine.Profile {
	return []engine.Profile{
		{
			Name:             "ripgrep",
			Description:      "Normalizes ripgrep into stable line-oriented output and groups matches by file.",
			Confidence:       engine.ConfidenceHigh,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isRipgrepCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareRipgrep(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeRipgrep(exec.Stdout+"\n"+exec.Stderr, maxGroups, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewRipgrepReducer(maxGroups, budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets plain-text `rg` usage and adds filename plus line-number flags when the user did not already request them.",
				"Groups matches by file and falls back to error-focused output when ripgrep fails instead of returning matches.",
			},
		},
		{
			Name:             "path-find",
			Description:      "Summarizes plain `find` output into a bounded match list.",
			Confidence:       engine.ConfidenceMedium,
			StreamPreference: engine.StreamStdoutFirst,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(25),
			Match: func(inv engine.Invocation) bool {
				return isPlainFindCommand(inv.Display)
			},
			Prepare: func(inv engine.Invocation) []string {
				return prepareFind(inv.Command)
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return filters.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return filters.NewFindReducer(budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Targets plain `find` usage that emits path lists without destructive actions or custom executors.",
				"Caps long file discovery output to a counted preview instead of replaying every path line.",
			},
		},
	}
}

func isRipgrepCommand(args []string) bool {
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	if profilekit.ContainsAny(args[1:], "--json", "--files", "--files-with-matches", "-l", "--count", "-c", "--count-matches") {
		return false
	}
	return true
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
		for _, glob := range filters.DefaultRipgrepExcludeGlobs() {
			injected = append(injected, "-g", glob)
		}
		out = append(injected, out[1:]...)
	}
	return out
}

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
	for _, dir := range filters.DefaultSearchNoiseDirs() {
		out = append(out, "-not", "-path", "*/"+dir+"/*")
	}
	return append(out, command[2:]...)
}

func shouldInjectDefaultRipgrepExcludes(args []string) bool {
	if len(args) == 0 {
		return false
	}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if consumesRipgrepValue(arg) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-g") || strings.HasPrefix(arg, "--glob") || strings.HasPrefix(arg, "--iglob") {
			return false
		}
		if arg == "--hidden" || arg == "--no-ignore" || arg == "--no-ignore-vcs" || arg == "--no-ignore-parent" || arg == "-u" || arg == "-uu" || arg == "-uuu" {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		positionals = append(positionals, arg)
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

func consumesRipgrepValue(arg string) bool {
	switch arg {
	case "-e", "-f", "-g", "--glob", "--iglob", "--ignore-file", "--max-filesize", "--type", "-t", "--type-not", "-T":
		return true
	default:
		return false
	}
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
