package engine

import (
	"strings"
	"time"
)

const (
	defaultTinyOutputBypassBytes  = 192
	defaultTinyOutputBypassTokens = 48
)

type FastPathDecision struct {
	BypassCompression bool
	BypassKind        string
	Reason            string
	WarnLatency       bool
}

const (
	FastPathBypassKindNone                 = ""
	FastPathBypassKindTinyOutput           = "tiny-output"
	FastPathBypassKindFamilyRule           = "family-rule"
	FastPathBypassKindEmptyPreferredStream = "empty-preferred-stream"
)

type fastPathRule struct {
	MaxBytes  int
	MaxTokens int
	Reason    string
}

var familyFastPathRules = map[string]fastPathRule{
	"ripgrep": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "tiny ripgrep output",
	},
	"path-find": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "tiny find output",
	},
	"generic-summary": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short summary output",
	},
	"directory-listing": {
		MaxBytes:  288,
		MaxTokens: 72,
		Reason:    "short directory listing",
	},
	"directory-listing-tree": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short tree output",
	},
	"grep": {
		MaxBytes:  320,
		MaxTokens: 80,
		Reason:    "tiny grep output",
	},
	"git-diff": {
		MaxBytes:  256,
		MaxTokens: 64,
		Reason:    "tiny git diff output",
	},
	"git-diff-names": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short git diff name list",
	},
	"git-log": {
		MaxBytes:  224,
		MaxTokens: 56,
		Reason:    "tiny git log output",
	},
	"git-ls-files": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short tracked file list",
	},
	"git-status": {
		MaxBytes:  192,
		MaxTokens: 48,
		Reason:    "short git status output",
	},
	"git-status-short": {
		MaxBytes:  288,
		MaxTokens: 72,
		Reason:    "short git status listing",
	},
	"ripgrep-files": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short ripgrep file list",
	},
	"ripgrep-files-with-matches": {
		MaxBytes:  384,
		MaxTokens: 96,
		Reason:    "short ripgrep matched-file list",
	},
}

func ResolveBudget(profile Profile, inv Invocation, fallbackLines int) OutputBudget {
	budget, _ := ResolveBudgetWithAdapter(profile, inv, fallbackLines, nil)
	return budget
}

func ResolveBudgetWithAdapter(profile Profile, inv Invocation, fallbackLines int, adapter BudgetAdapter) (OutputBudget, *BudgetAdaptation) {
	budget := profile.Budget
	if budget.MaxLines <= 0 {
		budget.MaxLines = fallbackLines
	}
	if budget.MaxLines <= 0 {
		budget.MaxLines = 12
	}
	if budget.MaxBytes <= 0 && budget.MaxLines > 0 {
		budget.MaxBytes = budget.MaxLines * 160
	}
	if budget.MaxTokens <= 0 && budget.MaxLines > 0 {
		budget.MaxTokens = budget.MaxLines * 32
	}
	budget.NoisePrefiltering = inv.Advanced.NoisePrefiltering
	budget.SemanticCompaction = inv.Advanced.SemanticCompaction
	budget.AdaptiveBudgets = inv.Advanced.AdaptiveBudgets
	budget.EarlyCaptureStop = inv.Advanced.EarlyCaptureStop
	budget.AggressiveRewrites = inv.Advanced.AggressivePrepareRewrites

	budget = tuneBudgetByProfile(profile, inv, budget)
	budget = tuneBudgetByReasoningMode(inv.ReasoningBudgetMode, budget)
	if inv.UltraCompact {
		budget = scaleBudget(budget, 3, 5)
	}
	switch {
	case inv.Verbose >= 2:
		budget = scaleBudget(budget, 3, 2)
	case inv.Verbose >= 1:
		budget = scaleBudget(budget, 5, 4)
	}
	switch profile.Confidence {
	case ConfidenceLow:
		budget = scaleBudget(budget, 5, 4)
	case ConfidenceMedium:
		budget = scaleBudget(budget, 9, 8)
	}

	var adaptation *BudgetAdaptation
	if adapter != nil && budget.AdaptiveBudgets {
		budget, adaptation = adapter.AdaptBudget(profile, inv, budget)
	}
	return finalizeResolvedBudget(budget), adaptation
}

func finalizeResolvedBudget(budget OutputBudget) OutputBudget {
	budget.MaxLines = clampInt(budget.MaxLines, 3, 40)
	if budget.MaxBytes <= 0 {
		budget.MaxBytes = budget.MaxLines * 160
	}
	if budget.MaxTokens <= 0 {
		budget.MaxTokens = budget.MaxLines * 32
	}
	return budget
}

func ExpandBudgetForFailureEscape(budget OutputBudget, inv Invocation) OutputBudget {
	if inv.UltraCompact {
		return scaleBudget(budget, 5, 4)
	}
	return scaleBudget(budget, 3, 2)
}

func DecideFastPath(profile Profile, inv Invocation, rawBytes, rawTokens int, duration time.Duration, exitCode int) FastPathDecision {
	decision := FastPathDecision{}
	if profile.LatencyBudget > 0 && duration > profile.LatencyBudget {
		decision.WarnLatency = true
	}
	if exitCode != 0 {
		return decision
	}
	if profile.StreamPreference == StreamStderrOnly && rawBytes == 0 {
		decision.BypassCompression = true
		decision.BypassKind = FastPathBypassKindEmptyPreferredStream
		decision.Reason = "stderr-only profile with empty stderr payload"
		return decision
	}
	if bypass, reason := commandFamilyMicroBypass(profile, inv, rawBytes, rawTokens); bypass {
		decision.BypassCompression = true
		decision.BypassKind = FastPathBypassKindFamilyRule
		decision.Reason = reason
		return decision
	}
	if rawBytes <= defaultTinyOutputBypassBytes && rawTokens <= defaultTinyOutputBypassTokens {
		decision.BypassCompression = true
		decision.BypassKind = FastPathBypassKindTinyOutput
		decision.Reason = "tiny output fast path"
	}
	return decision
}

func commandFamilyMicroBypass(profile Profile, inv Invocation, rawBytes, rawTokens int) (bool, string) {
	rule, ok := familyFastPathRules[fastPathRuleKey(profile, inv)]
	if !ok {
		return false, ""
	}
	if rawBytes <= rule.MaxBytes && rawTokens <= rule.MaxTokens {
		return true, rule.Reason
	}
	return false, ""
}

func fastPathRuleKey(profile Profile, inv Invocation) string {
	switch profile.Name {
	case "directory-listing":
		if isTreeShape(inv) {
			return "directory-listing-tree"
		}
	case "git-diff":
		if isGitDiffNameListShape(inv) {
			return "git-diff-names"
		}
	case "git-status":
		if isGitStatusShortShape(inv) {
			return "git-status-short"
		}
	case "generic-summary":
		if isGitLSFilesShape(inv) {
			return "git-ls-files"
		}
		if isRipgrepFilesShape(inv) {
			return "ripgrep-files"
		}
		if isRipgrepFilesWithMatchesShape(inv) {
			return "ripgrep-files-with-matches"
		}
	case "ripgrep-files":
		return "ripgrep-files"
	case "ripgrep-files-with-matches":
		return "ripgrep-files-with-matches"
	case "git-ls-files":
		return "git-ls-files"
	}
	return profile.Name
}

func isGitDiffNameListShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	if !hasCommand(args, "git", "diff") {
		return false
	}
	return containsAnyArg(args[2:], "--name-only", "--name-status")
}

func isGitLSFilesShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	return hasCommand(args, "git", "ls-files")
}

func isGitStatusShortShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	if !hasCommand(args, "git", "status") {
		return false
	}
	return containsAnyArg(args[2:], "--short", "--porcelain", "-s")
}

func isTreeShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	return len(args) > 0 && args[0] == "tree"
}

func isRipgrepFilesShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	return containsAnyArg(args[1:], "--files")
}

func isRipgrepFilesWithMatchesShape(inv Invocation) bool {
	args := effectiveFastPathArgs(inv)
	if len(args) == 0 || args[0] != "rg" {
		return false
	}
	return containsAnyArg(args[1:], "--files-with-matches", "-l")
}

func effectiveFastPathArgs(inv Invocation) []string {
	if len(inv.Command) > 0 {
		return inv.Command
	}
	return inv.Display
}

func hasCommand(args []string, head, sub string) bool {
	return len(args) >= 2 && args[0] == head && args[1] == sub
}

func containsAnyArg(args []string, needles ...string) bool {
	for _, arg := range args {
		for _, needle := range needles {
			if arg == needle {
				return true
			}
		}
	}
	return false
}

func tuneBudgetByProfile(profile Profile, inv Invocation, budget OutputBudget) OutputBudget {
	name := profile.Name
	if strings.Contains(name, "log") {
		return ensureBudgetLinesAtLeast(budget, 12)
	}
	if isCompactSummaryProfile(name) {
		return scaleBudget(budget, 4, 5)
	}
	if name == "js-workspace" {
		return tuneJSWorkspaceBudget(inv, budget)
	}
	if name == "python-tooling" {
		return scaleBudget(ensureBudgetLinesAtLeast(budget, 10), 9, 8)
	}
	if name == "pytest" {
		return scaleBudget(ensureBudgetLinesAtLeast(budget, 12), 5, 4)
	}
	if isTestOrBuildProfile(name) {
		return ensureBudgetLinesAtLeast(budget, 10)
	}
	return budget
}

func ensureBudgetLinesAtLeast(budget OutputBudget, minimum int) OutputBudget {
	if budget.MaxLines < minimum {
		budget.MaxLines = minimum
	}
	return budget
}

func isCompactSummaryProfile(name string) bool {
	switch name {
	case "ripgrep", "generic-summary", "git-log", "git-status":
		return true
	default:
		return false
	}
}

func tuneJSWorkspaceBudget(inv Invocation, budget OutputBudget) OutputBudget {
	if isPackageManagerWorkspaceCommand(inv) {
		budget = scaleBudget(budget, 3, 4)
	}
	if isTypeScriptCompilerCommand(inv) {
		budget = scaleBudget(ensureBudgetLinesAtLeast(budget, 12), 9, 8)
	}
	return budget
}

func isTestOrBuildProfile(name string) bool {
	return strings.Contains(name, "test") || strings.Contains(name, "build") || name == "ctest"
}

func isPackageManagerWorkspaceCommand(inv Invocation) bool {
	args := effectiveBudgetArgs(inv)
	if len(args) < 2 {
		return false
	}
	switch args[0] {
	case "npm", "pnpm", "yarn":
		return !(args[1] == "test" || (len(args) >= 3 && args[1] == "run" && args[2] == "test"))
	default:
		return false
	}
}

func isTypeScriptCompilerCommand(inv Invocation) bool {
	args := effectiveBudgetArgs(inv)
	if len(args) == 0 {
		return false
	}
	if args[0] == "tsc" {
		return true
	}
	return len(args) >= 2 && args[0] == "npx" && args[1] == "tsc"
}

func effectiveBudgetArgs(inv Invocation) []string {
	if len(inv.Command) > 0 {
		return CanonicalArgsForClassification(inv.Command)
	}
	return CanonicalArgsForClassification(inv.Display)
}

func tuneBudgetByReasoningMode(mode string, budget OutputBudget) OutputBudget {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "agent":
		budget = scaleBudget(budget, 3, 4)
		if budget.MinFailures < 1 {
			budget.MinFailures = 1
		}
		if budget.MinAnchors < 1 {
			budget.MinAnchors = 1
		}
		if budget.MinHints < 1 {
			budget.MinHints = 1
		}
	case "aggressive":
		budget = scaleBudget(budget, 1, 2)
		if budget.MinFailures < 1 {
			budget.MinFailures = 1
		}
		if budget.MinAnchors < 1 {
			budget.MinAnchors = 1
		}
		if budget.MinHints < 1 {
			budget.MinHints = 1
		}
	}
	return budget
}

func scaleBudget(budget OutputBudget, num, den int) OutputBudget {
	if num <= 0 || den <= 0 {
		return budget
	}
	budget.MaxLines = scaleIntCeil(budget.MaxLines, num, den)
	budget.MaxBytes = scaleIntCeil(budget.MaxBytes, num, den)
	budget.MaxTokens = scaleIntCeil(budget.MaxTokens, num, den)
	return budget
}

func scaleIntCeil(value, num, den int) int {
	if value <= 0 {
		return value
	}
	return (value*num + den - 1) / den
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
