package engine

import (
	"path/filepath"
	"strings"
)

func Classify(inv Invocation) Invocation {
	return classifyInvocation(inv)
}

func CanonicalArgsForClassification(args []string) []string {
	return canonicalArgsForClassification(args)
}

func classifyInvocation(inv Invocation) Invocation {
	inv.Classification = Classification{
		Command: classifyCommand(inv.Command),
		Display: classifyCommand(inv.Display),
	}
	return inv
}

func classifyCommand(args []string) ClassifiedCommand {
	classified := ClassifiedCommand{}
	canonical := canonicalArgsForClassification(args)
	if len(canonical) == 0 {
		return classified
	}

	classified.Head = canonical[0]
	if len(canonical) > 1 {
		classified.Subcommand = canonical[1]
	}
	classified.Git = classifyGitCommand(canonical)
	classified.JavaScript = classifyJavaScriptCommand(canonical)
	return classified
}

func classifyGitCommand(args []string) GitCommandFacts {
	args = canonicalGitArgs(args)
	if len(args) < 2 || args[0] != "git" {
		return GitCommandFacts{}
	}

	facts := GitCommandFacts{}
	rest := args[1:]
	switch args[1] {
	case "status":
		facts.StatusFormatRequested = containsAnyValue(rest, []string{"--short", "--porcelain", "-s"})
	case "log":
		facts.LogFormatRequested = containsAnyValue(rest, []string{"--oneline", "--stat", "-p"}) || containsPrefixedValue(rest, "--format")
	case "diff":
		facts.DiffFormatRequested = containsAnyValue(rest, []string{"--stat", "--numstat", "--shortstat", "--name-only", "--name-status"})
		facts.DiffNoPatchRequested = containsAnyValue(rest, []string{"--quiet", "--exit-code", "--no-patch", "-s"})
	}
	return facts
}

func canonicalArgsForClassification(args []string) []string {
	if len(args) == 0 {
		return nil
	}

	trimmed := stripCommandWrappers(args)
	if len(trimmed) == 0 {
		return nil
	}
	trimmed = stripNodeToolWrappers(trimmed)
	if len(trimmed) == 0 {
		return nil
	}

	out := append([]string{}, trimmed...)
	out[0] = filepath.Base(out[0])
	if out[0] == "git" {
		return canonicalGitArgs(out)
	}
	return out
}

func stripCommandWrappers(args []string) []string {
	trimmed := append([]string{}, args...)
	for len(trimmed) > 0 {
		switch trimmed[0] {
		case "env":
			trimmed = stripEnvPrefix(trimmed[1:])
		case "command", "builtin":
			trimmed = trimmed[1:]
		default:
			if isEnvAssignment(trimmed[0]) {
				trimmed = trimLeadingAssignments(trimmed)
				continue
			}
			return trimmed
		}
	}
	return trimmed
}

func stripEnvPrefix(args []string) []string {
	return stripWrapperPrefix(args, envWrapperOptions{})
}

func trimLeadingAssignments(args []string) []string {
	i := 0
	for i < len(args) && isEnvAssignment(args[i]) {
		i++
	}
	return args[i:]
}

func stripNodeToolWrappers(args []string) []string {
	trimmed := append([]string{}, args...)
	for len(trimmed) > 0 {
		switch filepath.Base(trimmed[0]) {
		case "npx":
			trimmed = stripNpxPrefix(trimmed[1:])
		default:
			return trimmed
		}
	}
	return trimmed
}

func stripNpxPrefix(args []string) []string {
	return stripWrapperPrefix(args, npxWrapperOptions{})
}

func isEnvAssignment(arg string) bool {
	if arg == "" || strings.HasPrefix(arg, "=") {
		return false
	}
	idx := strings.IndexByte(arg, '=')
	if idx <= 0 {
		return false
	}
	return !strings.ContainsAny(arg[:idx], "/\\")
}

func canonicalGitArgs(args []string) []string {
	if len(args) == 0 || args[0] != "git" {
		return args
	}

	out := []string{"git"}
	for i := 1; i < len(args); i++ {
		next, updatedOut, done := canonicalGitArgStep(args, i, out)
		out = updatedOut
		if done != nil {
			return done
		}
		i = next
	}
	return out
}

func canonicalGitArgStep(args []string, index int, out []string) (int, []string, []string) {
	arg := args[index]
	if arg == "--" {
		return index, out, append(out, args[index+1:]...)
	}
	if gitOptionConsumesValue(arg) {
		return skipGitOptionValue(args, index), out, nil
	}
	if gitInlineConfigOption(arg) || gitSkippableFlag(arg) {
		return index, out, nil
	}
	if strings.HasPrefix(arg, "-") {
		return index, append(out, arg), nil
	}
	return index, out, append(out, args[index:]...)
}

func skipGitOptionValue(args []string, index int) int {
	if index+1 >= len(args) {
		return len(args)
	}
	return index + 1
}

type wrapperOptionScanner interface {
	handle(arg string, remaining []string) (next int, done []string, handled bool)
}

type envWrapperOptions struct{}

func (envWrapperOptions) handle(arg string, remaining []string) (int, []string, bool) {
	switch {
	case isEnvAssignment(arg), arg == "-i", hasShortAttachedValue(arg, "-u"):
		return 1, nil, true
	case arg == "-u":
		if len(remaining) < 2 {
			return 0, nil, true
		}
		return 2, nil, true
	case strings.HasPrefix(arg, "-"):
		return 1, nil, true
	default:
		return 0, nil, false
	}
}

type npxWrapperOptions struct{}

func (npxWrapperOptions) handle(arg string, remaining []string) (int, []string, bool) {
	switch {
	case npxOptionConsumesValue(arg):
		if len(remaining) < 2 {
			return 0, nil, true
		}
		return 2, nil, true
	case npxInlineValueOption(arg), npxFlagWithoutValue(arg):
		return 1, nil, true
	case strings.HasPrefix(arg, "-"):
		return 1, nil, true
	default:
		return 0, nil, false
	}
}

func stripWrapperPrefix(args []string, scanner wrapperOptionScanner) []string {
	for i := 0; i < len(args); {
		arg := args[i]
		if arg == "--" {
			return wrapperCommandAfterSeparator(args, i)
		}
		if next, done, handled := scanner.handle(arg, args[i:]); handled {
			if done != nil || next == 0 {
				return done
			}
			i += next
			continue
		}
		return args[i:]
	}
	return nil
}

func wrapperCommandAfterSeparator(args []string, index int) []string {
	if index+1 < len(args) {
		return args[index+1:]
	}
	return nil
}

func hasShortAttachedValue(arg string, prefix string) bool {
	return strings.HasPrefix(arg, prefix) && len(arg) > len(prefix)
}

func npxOptionConsumesValue(arg string) bool {
	switch arg {
	case "-p", "--package", "-c", "--call", "--node-options", "--shell", "--userconfig":
		return true
	default:
		return false
	}
}

func npxInlineValueOption(arg string) bool {
	for _, prefix := range []string{"--package=", "--call=", "--node-options=", "--shell=", "--userconfig="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func npxFlagWithoutValue(arg string) bool {
	switch arg {
	case "--yes", "--no", "-y", "-q", "--quiet", "--ignore-existing":
		return true
	default:
		return false
	}
}

func gitOptionConsumesValue(arg string) bool {
	switch arg {
	case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix", "--config-env":
		return true
	default:
		return false
	}
}

func gitInlineConfigOption(arg string) bool {
	for _, prefix := range []string{"--git-dir=", "--work-tree=", "--namespace=", "--super-prefix=", "--config-env="} {
		if strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func gitSkippableFlag(arg string) bool {
	switch arg {
	case "-p", "--paginate", "-P", "--no-pager", "--no-replace-objects", "--bare", "--literal-pathspecs", "--no-literal-pathspecs", "--glob-pathspecs", "--noglob-pathspecs", "--icase-pathspecs":
		return true
	default:
		return false
	}
}

func classifyJavaScriptCommand(args []string) JavaScriptCommandFacts {
	facts := JavaScriptCommandFacts{}
	if len(args) == 0 {
		return facts
	}

	facts.IsPackageManagerTest = isPackageManagerTestCommand(args)
	facts.IsWorkspaceCommand = isJavaScriptWorkspaceCommand(args)
	facts.Runner = detectJavaScriptRunner(args)
	if facts.Runner != "" {
		facts.StructuredMode = hasStructuredJavaScriptArgs(args, facts.Runner)
	}
	return facts
}

func isPackageManagerTestCommand(args []string) bool {
	return len(args) >= 2 &&
		(args[0] == "npm" || args[0] == "pnpm" || args[0] == "yarn") &&
		(args[1] == "test" || len(args) >= 3 && args[1] == "run" && args[2] == "test")
}

func isJavaScriptWorkspaceCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn":
		return !isPackageManagerTestCommand(args)
	case "biome", "bun", "esbuild", "eslint", "next", "node", "nuxt", "nx", "rollup", "swc", "swc-cli", "ts-node", "tsc", "tsx", "tsup", "turbo", "vite", "webpack":
		if args[0] != "node" {
			return true
		}
		return len(args) >= 2 && (strings.HasSuffix(args[1], ".mjs") || strings.HasSuffix(args[1], ".cjs"))
	default:
		return false
	}
}

func detectJavaScriptRunner(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch args[0] {
	case "jest", "vitest":
		return args[0]
	case "bun":
		if len(args) >= 2 && args[1] == "test" {
			return "bun"
		}
	}

	script := strings.ToLower(strings.Join(args, " "))
	switch {
	case strings.Contains(script, "--runinband"):
		return "jest"
	case strings.Contains(script, "vitest"):
		return "vitest"
	case strings.Contains(script, "jest"):
		return "jest"
	default:
		return ""
	}
}

func hasStructuredJavaScriptArgs(args []string, runner string) bool {
	if runner == "jest" {
		return containsAnyValue(args, []string{"--json"}) || containsPrefixedValue(args, "--outputFile") || containsPrefixedValue(args, "--reporters")
	}
	if runner == "vitest" {
		return containsAnyValue(args, []string{"--reporter=json"}) || containsPrefixedValue(args, "--reporter=") || containsPrefixedValue(args, "--outputFile")
	}
	return false
}

func containsPrefixedValue(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
