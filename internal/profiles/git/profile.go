package git

import (
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
	logSummary := profilekit.StdoutSummary(maxLines, 11, 15, engine.StreamStdoutOnly, func(stdout string) string {
		return gitfilter.SummarizeGitLog(shared.StripANSI(stdout))
	}, func(budget engine.OutputBudget) engine.StreamReducer {
		return gitfilter.NewGitLogReducer(budget.MaxLines, budget.MaxBytes)
	})

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
				return append(inv.Command, "--oneline", "-n", "20")
			},
			Explain: []string{
				"Injects `--oneline -n 20` for plain `git log` calls.",
				"Keeps the preview shallow so the LLM sees commit shape instead of full message bodies.",
			},
		}, logSummary),
		{
			Name:        "git-diff",
			Description: "Summarizes file churn and preserves `--stat` style detail.",
			Confidence:  engine.ConfidenceHigh,
			Capabilities: engine.ProfileCapabilities{
				StructuredMode:            engine.StructuredModePreferred,
				InjectsPrepareArgs:        true,
				SupportsAggressivePrepare: true,
			},
			StreamPreference: engine.StreamStdoutOnly,
			Budget:           profilekit.OutputBudget(profilekit.AtLeast(maxLines, 9)),
			LatencyBudget:    profilekit.LatencyBudget(20),
			Match: func(inv engine.Invocation) bool {
				return inv.Classification.Display.Head == "git" && inv.Classification.Display.Subcommand == "diff"
			},
			Prepare: func(inv engine.Invocation) []string {
				if inv.Classification.Command.Git.DiffNoPatchRequested {
					return inv.Command
				}
				if !inv.Advanced.AggressivePrepareRewrites {
					if inv.Classification.Command.Git.DiffFormatRequested {
						return ensureGitDiffNoiseFlags(inv.Command)
					}
					return ensureGitDiffNoiseFlags(append(inv.Command, "--stat=96,20", "--compact-summary"))
				}
				if inv.Classification.Command.Git.DiffFormatRequested {
					return ensureGitDiffNoiseFlags(inv.Command)
				}
				if isAggressiveGitDiff(inv) {
					return ensureGitDiffNoiseFlags(append(inv.Command, "--stat=56,8", "--compact-summary"))
				}
				return ensureGitDiffNoiseFlags(append(inv.Command, "--stat=72,12", "--compact-summary"))
			},
			Render: func(inv engine.Invocation, exec engine.Execution) string {
				return newGitDiffReducer(inv, maxLines, 0).Reduce(shared.StripANSI(exec.Stdout))
			},
			StreamRender: func(inv engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
				return newGitDiffReducer(inv, budget.MaxLines, budget.MaxBytes)
			},
			ParseBytes: profilekit.ParseStdout,
			Explain: []string{
				"Biases `git diff` toward stat output instead of full hunks, with narrower stat widths in aggressive mode.",
				"Totals additions and deletions, then keeps the highest-churn files when the diff touches many paths.",
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
			"Normalizes summary-style `git show` invocations toward concise headers and suppresses patch noise when the user already asked for stat or name-only output.",
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

func newGitDiffReducer(inv engine.Invocation, maxLines int, maxBytes int) *gitfilter.GitDiffReducer {
	return gitfilter.NewGitDiffReducerWithOptions(gitfilter.GitDiffReducerOptions{
		MaxLines:              maxLines,
		MaxBytes:              maxBytes,
		Aggressive:            isAggressiveGitDiff(inv),
		LargeFileThreshold:    5,
		LargeSummaryTopN:      4,
		AggressiveSummaryTopN: 2,
	})
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
	if !profilekit.ContainsAny(command[1:], "--no-patch", "-s") {
		out = append(out, "--no-patch")
	}
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
