package git

import (
	"github.com/devr-tools/szr/internal/engine"
	shared "github.com/devr-tools/szr/internal/filters"
	gitfilter "github.com/devr-tools/szr/internal/filters/git"
	"github.com/devr-tools/szr/internal/profilekit"
)

type gitOpSpec struct {
	name        string
	description string
	subcommands []string
	preference  string
	explain     []string
}

//nolint:maintidx // Profile specs are declarative and intentionally keep match/render metadata together.
func opsProfiles(maxLines int) []engine.Profile {
	specs := []gitOpSpec{
		{
			name:        "git-fetch",
			description: "Keeps fetched ref updates and drops object transfer progress.",
			subcommands: []string{"fetch"},
			preference:  engine.StreamStderrFirst,
			explain: []string{
				"Suppresses `Receiving objects`, `Resolving deltas`, and `remote:` progress chatter from git fetch.",
				"Keeps ref update lines and errors, and folds transfer counts into a single objects/deltas summary.",
			},
		},
		{
			name:        "git-clone",
			description: "Reduces clone output to the target directory and final object counts.",
			subcommands: []string{"clone"},
			preference:  engine.StreamStderrFirst,
			explain: []string{
				"Keeps `Cloning into ...`, checkout warnings, and errors from git clone.",
				"Collapses object enumeration, compression, and delta progress into one summary line.",
			},
		},
		{
			name:        "git-merge",
			description: "Preserves merge outcomes and every conflict while folding per-file chatter.",
			subcommands: []string{"merge"},
			preference:  engine.StreamStdoutFirst,
			explain: []string{
				"Keeps fast-forward/merge summaries, change totals, and all `CONFLICT` lines from git merge.",
				"Folds repeated `Auto-merging` lines into a single preview so conflicts stay visible.",
			},
		},
		{
			name:        "git-rebase",
			description: "Keeps rebase outcomes, conflicts, and error/hint guidance only.",
			subcommands: []string{"rebase"},
			preference:  engine.StreamStdoutFirst,
			explain: []string{
				"Keeps `Successfully rebased`, `CONFLICT`, and `error:`/`hint:` lines from git rebase.",
				"Drops per-commit application chatter and progress noise.",
			},
		},
		{
			name:        "git-checkout",
			description: "Compresses checkout and switch output to branch state and warnings.",
			subcommands: []string{"checkout", "switch"},
			preference:  engine.StreamStderrFirst,
			explain: []string{
				"Keeps `Switched to ...`, branch tracking state, and detached-HEAD warnings from git checkout/switch.",
				"Suppresses `Updating files` progress and long advice blocks beyond the budget.",
			},
		},
		{
			name:        "git-reset",
			description: "Keeps the resulting HEAD position and truncates per-file reset chatter.",
			subcommands: []string{"reset"},
			preference:  engine.StreamStdoutFirst,
			explain: []string{
				"Keeps `HEAD is now at ...` and error lines from git reset.",
				"Caps the unstaged-change file listing instead of replaying every path.",
			},
		},
		{
			name:        "git-stash",
			description: "Keeps stash identifiers and outcomes while capping file listings.",
			subcommands: []string{"stash"},
			preference:  engine.StreamStdoutFirst,
			explain: []string{
				"Keeps `Saved working directory ...`, `Dropped ...`, and `stash@{n}` identifiers from git stash.",
				"Caps status-style file chatter emitted by stash pop/apply.",
			},
		},
		{
			name:        "git-cherry-pick",
			description: "Keeps cherry-pick results, conflicts, and resolution hints.",
			subcommands: []string{"cherry-pick"},
			preference:  engine.StreamStdoutFirst,
			explain: []string{
				"Keeps the applied commit summary and change totals from git cherry-pick.",
				"Always retains `CONFLICT`, `error:`, and `hint:` lines so failed picks stay actionable.",
			},
		},
	}

	out := make([]engine.Profile, 0, len(specs)+1)
	for _, spec := range specs {
		out = append(out, gitOpProfile(spec, maxLines))
	}
	out = append(out, gitBranchProfile(maxLines))
	return out
}

func gitOpProfile(spec gitOpSpec, maxLines int) engine.Profile {
	kind := spec.subcommands[0]
	budgetLines := profilekit.AtLeast(maxLines, 8)
	return engine.Profile{
		Name:        spec.name,
		Description: spec.description,
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: spec.preference,
		Budget:           profilekit.OutputBudget(budgetLines),
		LatencyBudget:    profilekit.LatencyBudget(20),
		Match:            matchGitSubcommands(spec.subcommands...),
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitOp(kind, exec.Stdout+"\n"+exec.Stderr, budgetLines)
		},
		StreamRender: gitOpStreamRender(kind),
		ParseBytes:   profilekit.ParseCombined,
		Explain:      spec.explain,
	}
}

func gitOpStreamRender(kind string) func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
	return func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
		return shared.NewBufferedTextReducerWithRecovery(true, true, func(input string) string {
			return gitfilter.SummarizeGitOp(kind, input, budget.MaxLines)
		}, func(input string) (string, string, bool) {
			return gitfilter.GitOpRecoveryInfo(kind, input, budget.MaxLines)
		})
	}
}

//nolint:maintidx // Profile constructors are declarative and intentionally keep match/render behavior together.
func gitBranchProfile(maxLines int) engine.Profile {
	budgetLines := profilekit.AtLeast(maxLines, 8)
	return engine.Profile{
		Name:        "git-branch",
		Description: "Groups long branch listings into a bounded, bucketed preview.",
		Confidence:  engine.ConfidenceHigh,
		Capabilities: engine.ProfileCapabilities{
			AllowFailureEscape: true,
		},
		StreamPreference: engine.StreamStdoutFirst,
		Budget:           profilekit.OutputBudget(budgetLines),
		LatencyBudget:    profilekit.LatencyBudget(15),
		Match:            matchGitSubcommands("branch"),
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return gitfilter.SummarizeGitBranches(exec.Stdout+"\n"+exec.Stderr, budgetLines)
		},
		StreamRender: func(_ engine.Invocation, budget engine.OutputBudget) engine.StreamReducer {
			return shared.NewBufferedTextReducerWithRecovery(true, true, func(input string) string {
				return gitfilter.SummarizeGitBranches(input, budget.MaxLines)
			}, func(input string) (string, string, bool) {
				return gitBranchRecoveryInfo(input, budget.MaxLines)
			})
		},
		ParseBytes: profilekit.ParseCombined,
		Explain: []string{
			"Passes short branch listings through untouched and marks the current branch in long ones.",
			"Groups many branches into prefix buckets (like directory previews) instead of listing every name.",
		},
	}
}

func gitBranchRecoveryInfo(input string, maxLines int) (string, string, bool) {
	rawLines := shared.NonEmptyLines(shared.StripANSI(input))
	renderedLines := shared.NonEmptyLines(gitfilter.SummarizeGitBranches(input, maxLines))
	if len(rawLines) == 0 || len(renderedLines) >= len(rawLines) {
		return shared.NoRecovery()
	}
	return shared.FullOutputRecovery("omitted full branch listing")
}

func matchGitSubcommands(names ...string) func(engine.Invocation) bool {
	return func(inv engine.Invocation) bool {
		if inv.Classification.Display.Head != "git" {
			return false
		}
		for _, name := range names {
			if inv.Classification.Display.Subcommand == name {
				return true
			}
		}
		return false
	}
}
