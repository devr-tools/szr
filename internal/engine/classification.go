package engine

import "strings"

func Classify(inv Invocation) Invocation {
	return classifyInvocation(inv)
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
	if len(args) == 0 {
		return classified
	}

	classified.Head = args[0]
	if len(args) > 1 {
		classified.Subcommand = args[1]
	}
	classified.Git = classifyGitCommand(args)
	classified.JavaScript = classifyJavaScriptCommand(args)
	return classified
}

func classifyGitCommand(args []string) GitCommandFacts {
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
