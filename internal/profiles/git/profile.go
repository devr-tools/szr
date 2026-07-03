package git

import (
	"strconv"
	"strings"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	fsfilter "github.com/devr-tools/szr/internal/filters/fs"
	gitfilter "github.com/devr-tools/szr/internal/filters/git"
	"github.com/devr-tools/szr/internal/profilekit"
)

func Profiles(maxLines int) []engine.Profile {
	statusSummary := profilekit.StdoutSummary(maxLines, 8, 15, engine.StreamStdoutOnly, func(stdout string) string {
		return gitfilter.SummarizeGitStatus(shared.StripANSI(stdout))
	}, func(budget engine.OutputBudget) engine.StreamReducer {
		return gitfilter.NewGitStatusReducer(budget.MaxLines, budget.MaxBytes)
	})
	logBudget := profilekit.OutputBudget(profilekit.AtLeast(maxLines, 11))
	logSummary := profilekit.SummaryConfig{
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           logBudget,
		LatencyBudget:    15,
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			reducer := newGitLogReducer(inv, logBudget)
			reducer.ConsumeStdout([]byte(shared.StripANSI(exec.Stdout)))
			return reducer.Result()
		},
		StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return newGitLogReducer(inv, budget)
		},
		ParseBytes: profilekit.ParseStdout,
	}

	list := []engine.Profile{
		gitShowProfile(maxLines),
		gitAddProfile(maxLines),
		gitCommitProfile(maxLines),
		gitPushProfile(maxLines),
		gitPullProfile(maxLines),
		{
			Name:        "git-ls-files",
			Description: "Summarizes tracked file lists into a bounded path preview.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
			},
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 8)),
			LatencyBudget:    profilekit.LatencyBudget(15),
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "ls-files"
			},
			Prepare: func(inv engine.Invocation) []string {
				return inv.Command
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return shared.SummarizeFindOutput(exec.Stdout, exec.Stderr, maxLines)
			},
			StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return shared.NewFindReducer(budget.MaxLines)
			},
			ParseBytes: profilekit.ParseCombined,
			Explain: []string{
				"Matches `git ls-files` directly instead of routing file lists through a generic fallback.",
				"Summarizes tracked paths as a bounded, deduplicated list with the same path-list behavior used for other discovery commands.",
			},
		},
		profilekit.WithSummary(engine.Profile{
			Name:        "git-status",
			Description: "Condenses git working tree state into branch and file counts.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:            engine.FastPathBypassSafeOnly,
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "status"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.StatusFormatRequested {
					return inv.Command
				}
				return append(inv.Command, "--short", "--branch")
			},
			Explain: []string{
				"Rewrites `git status` into `git status --short --branch` unless a machine-readable mode was already requested.",
				"Extracts branch, staged, unstaged, and untracked counts with a short file preview.",
			},
		}, statusSummary),
		profilekit.WithSummary(engine.Profile{
			Name:        "git-log",
			Description: "Prefers oneline commit output and trims the history preview.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				FastPathBypass:     engine.FastPathBypassSafeOnly,
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
			},
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "log"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.LogFormatRequested {
					return inv.Command
				}
				if _, ok := gitLogRequestedCount(inv.Command); ok {
					return inv.Command
				}
				return append(inv.Command, "--oneline", "-n", "20")
			},
			Explain: []string{
				"Injects `--oneline -n 20` only when `git log` was called without an explicit count or format flag.",
				"Explicit `-<n>`, `-n`, `--max-count`, and format flags always win; the summary then keeps up to the requested number of commits.",
			},
		}, logSummary),
		{
			Name:        "git-diff",
			Description: "Summarizes diff output into per-file churn with full filenames.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:     engine.StructuredModePreferred,
				InjectsPrepareArgs: true,
			},
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           gitDiffBudget(maxLines),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "diff"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.DiffNoPatchRequested {
					return inv.Command
				}
				return ensureGitDiffNoiseFlags(inv.Command)
			},
			Render: func(inv engine.Invocation, exec engine.Execution) string {
				return newGitDiffReducer(inv, maxLines, 0).Reduce(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newGitDiffReducerWithTokens(inv, budget.MaxLines, budget.MaxBytes, budget.MaxTokens)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Runs the user's diff arguments unchanged (plus `--no-color --no-ext-diff`) and summarizes the captured patch at render time.",
				"Keeps full file names with per-file hunk and +/- counts, marks conflicted paths, and keeps the highest-churn files when the diff touches many paths.",
			},
		},
	}
	return append(list, opsProfiles(maxLines)...)
}

//nolint:maintidx // Profile constructors are declarative and intentionally keep match/render behavior together.
func gitShowProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:        "git-show",
		Description: "Summarizes commit previews, file-level diffs, and blob reads from git show.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			StructuredMode:            engine.StructuredModePreferred,
			InjectsPrepareArgs:        true,
			SupportsAggressivePrepare: true,
		},
		StreamPreference: engine.StreamStdoutOnly,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 10)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchGitSubcommand("show"),
		Prepare: func(inv engine.Invocation) []string {
			return prepareGitShowCommand(inv.Command)
		},
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return renderGitShow(inv, exec.Stdout, maxLines)
		},
		StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(true, false, func(input string) string {
				return renderGitShow(inv, input, budget.MaxLines)
			}, func(input string) (string, string, bool) {
				return gitShowRecoveryInfo(inv, input, budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseStdout,
		Explain: []string{
			"Matches repeated `git show` inspection patterns from local history, including summary-oriented commit previews and `REV:path` blob reads.",
			"Compacts the commit header with `--format=oneline` for summary-style invocations while leaving user-requested stat/name-only output intact.",
		},
	}
}

func gitAddProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:        "git-add",
		Description: "Collapses successful staging commands into a path-aware one-liner.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(15),
		Match:            matchGitSubcommand("add"),
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitAdd(inv, exec.Stdout+"\n"+exec.Stderr)
		},
		StreamRender: gitSuccessPathStreamRender("add"),
		ParseBytes:   profilekit.ParseCombined,
		Explain: []string{
			"Reduces successful `git add` runs to whether changes were staged and which paths were targeted.",
			"Falls back on any unexpected output so warnings and failures still surface verbatim.",
		},
	}
}

func gitCommitProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:        "git-commit",
		Description: "Compresses successful commits to hash, subject, and top-level change totals.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchGitSubcommand("commit"),
		Render: func(inv engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitCommit(inv, exec.Stdout+"\n"+exec.Stderr)
		},
		StreamRender: gitSuccessPathStreamRender("commit"),
		ParseBytes:   profilekit.ParseCombined,
		Explain: []string{
			"Extracts the commit identity the agent actually needs instead of preserving per-file mode chatter.",
			"Unknown output shapes fall back so `nothing to commit`, hook failures, and merge conflicts stay explicit.",
		},
	}
}

func gitPushProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:        "git-push",
		Description: "Condenses successful pushes to the updated ref or up-to-date status.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStderrFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchGitSubcommand("push"),
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitPush(exec.Stdout + "\n" + exec.Stderr)
		},
		StreamRender: gitSuccessPathStreamRender("push"),
		ParseBytes:   profilekit.ParseCombined,
		Explain: []string{
			"Prefers the ref-level push result over object-count progress lines and remote boilerplate.",
			"Leaves unrecognized push output on the fallback path so rejects and auth failures are preserved.",
		},
	}
}

func gitPullProfile(maxLines int) engine.Profile {
	return engine.Profile{
		Name:        "git-pull",
		Description: "Shrinks successful pulls to up-to-date, fast-forward, merge, or rebase outcomes.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 6)),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchGitSubcommand("pull"),
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitPull(exec.Stdout + "\n" + exec.Stderr)
		},
		StreamRender: gitSuccessPathStreamRender("pull"),
		ParseBytes:   profilekit.ParseCombined,
		Explain: []string{
			"Extracts the synchronization outcome instead of keeping fetch noise and full file lists.",
			"Falls back when pull output does not match a known success path so conflict details remain intact.",
		},
	}
}

func matchGitSubcommand(name string) func(engine.Invocation) bool {
	return func(inv engine.Invocation) bool {
		return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == name
	}
}

func gitSuccessPathStreamRender(kind string) func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
	return func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
		return gitfilter.NewGitSuccessPathReducer(kind, inv, budget.MaxLines, budget.MaxBytes)
	}
}

func newGitLogReducer(inv engine.Invocation, budget engine.OutputBudget) *gitfilter.GitLogReducer {
	maxEntries := 0
	if count, ok := gitLogRequestedCount(inv.Command); ok {
		maxEntries = count
	} else if count, ok := gitLogRequestedCount(inv.Display); ok {
		maxEntries = count
	}
	if limit := budget.MaxLines - 1; limit > 0 && maxEntries > limit {
		maxEntries = limit
	}
	return gitfilter.NewGitLogReducerWithEntries(budget.MaxLines, budget.MaxBytes, maxEntries)
}

// gitLogRequestedCount reports an explicit commit count requested by the user
// via `-<n>`, `-n <n>`, `-n<n>`, `--max-count <n>`, or `--max-count=<n>`.
func gitLogRequestedCount(command []string) (int, bool) {
	canonical := engine.CanonicalArgsForClassification(command)
	if len(canonical) < 3 || canonical[0] != "git" || canonical[1] != "log" {
		return 0, false
	}
	rest := canonical[2:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if arg == "--" {
			break
		}
		if count, ok := parseGitLogCountArg(arg, rest, i); ok {
			return count, true
		}
	}
	return 0, false
}

func parseGitLogCountArg(arg string, rest []string, index int) (int, bool) {
	switch {
	case arg == "-n" || arg == "--max-count":
		if index+1 < len(rest) {
			return parseNonNegativeInt(rest[index+1])
		}
		return 0, false
	case strings.HasPrefix(arg, "--max-count="):
		return parseNonNegativeInt(strings.TrimPrefix(arg, "--max-count="))
	case strings.HasPrefix(arg, "-n") && len(arg) > 2:
		return parseNonNegativeInt(arg[2:])
	case len(arg) > 1 && arg[0] == '-':
		return parseNonNegativeInt(arg[1:])
	default:
		return 0, false
	}
}

func parseNonNegativeInt(value string) (int, bool) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return 0, false
	}
	return parsed, true
}

func newGitDiffReducer(inv engine.Invocation, maxLines int, maxBytes int) *gitfilter.GitDiffReducer {
	return newGitDiffReducerWithTokens(inv, maxLines, maxBytes, gitDiffBudget(maxLines).MaxTokens)
}

func newGitDiffReducerWithTokens(inv engine.Invocation, maxLines, maxBytes, maxTokens int) *gitfilter.GitDiffReducer {
	return gitfilter.NewGitDiffReducerWithOptions(gitfilter.GitDiffReducerOptions{
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
		MaxTokens:             maxTokens,
		Aggressive:            isAggressiveGitDiff(inv),
		LargeFileThreshold:    5,
		LargeSummaryTopN:      4,
		AggressiveSummaryTopN: 2,
	})
}

// gitDiffBudget widens the standard token budget for git-diff renders: a
// many-file diff has to account for every touched filename (the inventory
// render), and that payload cannot fit the generic per-profile token cap.
// The compression contract's raw-tokens/5 ceiling still applies, so small
// diffs are unaffected.
func gitDiffBudget(maxLines int) engine.OutputBudget {
	budget := profilekit.OutputBudget(profilekit.AtLeast(maxLines, 9))
	budget.MaxTokens *= 5
	budget.MaxBytes *= 5
	return budget
}

func isAggressiveGitDiff(inv engine.Invocation) bool {
	return inv.UltraCompact || inv.ReasoningBudgetMode == config.ReasoningBudgetAggressive
}

func ensureGitDiffNoiseFlags(command []string) []string {
	out := append([]string{}, command...)
	if !profilekit.ContainsAny(command[1:], "--no-color", "--color=never") && !profilekit.ContainsPrefix(command[1:], "--color=") {
		out = append(out, "--no-color")
	}
	if !profilekit.ContainsAny(command[1:], "--no-ext-diff", "--ext-diff") {
		out = append(out, "--no-ext-diff")
	}
	return out
}

//nolint:maintidx // The prepare logic mirrors git-show modes directly so flag injection stays predictable.
func prepareGitShowCommand(command []string) []string {
	if len(command) == 0 {
		return command
	}

	out := append([]string{}, command...)
	if !profilekit.ContainsAny(command[1:], "--no-color", "--color=never") && !profilekit.ContainsPrefix(command[1:], "--color=") {
		out = append(out, "--no-color")
	}
	if !profilekit.ContainsAny(command[1:], "--no-ext-diff", "--ext-diff") {
		out = append(out, "--no-ext-diff")
	}

	if gitShowBlobPath(command) != "" {
		return out
	}
	if !gitShowSummaryRequested(command) {
		return out
	}
	// Never append --no-patch here: it would suppress the summary output the
	// user explicitly requested (for example `git show --stat`).
	if !gitShowPrettyRequested(command) {
		out = append(out, "--format=oneline")
	}
	return out
}

//nolint:maintidx // Rendering keeps blob previews and commit summaries together for one reducer entrypoint.
func renderGitShow(inv engine.Invocation, stdout string, maxLines int) string {
	clean := shared.StripANSI(stdout)
	if clean == "" {
		return ""
	}

	if blobPath := gitShowBlobPath(inv.Command); blobPath != "" {
		return fsfilter.SummarizeReadFile(blobPath, []byte(clean), maxLines)
	}
	if blobPath := gitShowBlobPath(inv.Display); blobPath != "" {
		return fsfilter.SummarizeReadFile(blobPath, []byte(clean), maxLines)
	}

	headline, body := splitGitShowOutput(clean)
	if headline == "" {
		return newGitDiffReducer(inv, maxLines, 0).Reduce(clean)
	}
	if body == "" || maxLines <= 1 {
		return headline
	}

	bodySummary := summarizeGitShowBody(inv, body, maxLines-1)
	if bodySummary == "" {
		return headline
	}
	return headline + "\n" + bodySummary
}

func gitShowRecoveryInfo(inv engine.Invocation, stdout string, maxLines int) (string, string, bool) {
	rawLines := shared.NonEmptyLines(shared.StripANSI(stdout))
	renderedLines := shared.NonEmptyLines(renderGitShow(inv, stdout, maxLines))
	if len(rawLines) == 0 || len(renderedLines) >= len(rawLines) {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted git show details")
}

//nolint:maintidx // Output splitting is intentionally explicit because git show formats vary widely.
func splitGitShowOutput(input string) (string, string) {
	lines := strings.Split(input, "\n")
	headline := ""
	bodyStart := -1
	for idx, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if headline == "" {
			headline = summarizeGitShowHeadline(trimmed)
			continue
		}
		if isGitShowBodyLine(trimmed) {
			bodyStart = idx
			break
		}
	}
	if headline == "" {
		return "", ""
	}
	if bodyStart == -1 {
		return headline, ""
	}
	return headline, strings.Join(lines[bodyStart:], "\n")
}

func summarizeGitShowHeadline(line string) string {
	if !strings.HasPrefix(line, "commit ") {
		return line
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return line
	}
	hash := fields[1]
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return "show " + hash
}

func summarizeGitShowBody(inv engine.Invocation, body string, maxLines int) string {
	lines := shared.NonEmptyLines(body)
	if len(lines) == 0 {
		return ""
	}

	if paths, ok := extractGitShowPaths(lines); ok {
		return shared.SummarizeFindOutput(strings.Join(paths, "\n"), "", maxLines)
	}

	summary := newGitDiffReducer(inv, maxLines, 0).Reduce(body)
	if summary != "" && summary != "no diff" {
		return summary
	}
	return shared.JoinLimitedLines(lines, maxLines)
}

func extractGitShowPaths(lines []string) ([]string, bool) {
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if idx := strings.IndexByte(trimmed, '\t'); idx > 0 && idx <= 2 {
			paths = append(paths, trimmed[idx+1:])
			continue
		}
		if strings.Contains(trimmed, " ") || strings.Contains(trimmed, "|") {
			return nil, false
		}
		paths = append(paths, trimmed)
	}
	return paths, len(paths) > 0
}

func isGitShowBodyLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "diff --git "),
		strings.HasPrefix(line, "@@ "),
		strings.Contains(line, " files changed"),
		strings.Contains(line, " file changed"),
		strings.Contains(line, " | "):
		return true
	case strings.ContainsRune(line, '\t'):
		return true
	case !strings.HasPrefix(line, "Author:") && !strings.HasPrefix(line, "Date:") && !strings.HasPrefix(line, "commit "):
		return !strings.HasPrefix(line, "    ")
	default:
		return false
	}
}

func gitShowSummaryRequested(command []string) bool {
	canonical := engine.CanonicalArgsForClassification(command)
	if len(canonical) < 2 {
		return false
	}
	for _, arg := range canonical[2:] {
		if arg == "--" {
			return false
		}
		if arg == "--stat" || arg == "--name-only" || arg == "--name-status" || arg == "--no-patch" || strings.HasPrefix(arg, "--stat=") {
			return true
		}
	}
	return false
}

func gitShowPrettyRequested(command []string) bool {
	canonical := engine.CanonicalArgsForClassification(command)
	if len(canonical) < 2 {
		return false
	}
	for _, arg := range canonical[2:] {
		if arg == "--" {
			return false
		}
		if arg == "--oneline" || arg == "--format" || arg == "--pretty" || strings.HasPrefix(arg, "--format=") || strings.HasPrefix(arg, "--pretty=") {
			return true
		}
	}
	return false
}

func gitShowBlobPath(command []string) string {
	canonical := engine.CanonicalArgsForClassification(command)
	if len(canonical) < 3 || canonical[0] != "git" || canonical[1] != "show" {
		return ""
	}
	for _, arg := range canonical[2:] {
		if arg == "--" {
			return ""
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if idx := strings.LastIndex(arg, ":"); idx > 0 && idx+1 < len(arg) {
			return arg[idx+1:]
		}
	}
	return ""
}
