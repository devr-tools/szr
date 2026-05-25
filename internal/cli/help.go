package cli

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var compactBanner = []string{
	"⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⣤⣤⣤⡀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⢠⣾⣿⣿⣿⣿⣿⣧⡀⠀⠀⠀⠀",
	"⠀⠀⠀⠀⠀⠀⢸⣿⣿⣿⠛⣿⣿⠛⡇⠀⠙⠀",
	"⠀⠈⠉⠉⠁⠀⣀⣈⢿⣿⣿⣶⣿⣿⡶⠃⣴⣦⠀",
	"⠀⠀⠀⢀⣴⣶⣿⣿⣮⣝⡻⠿⠿⣛⣁⣠⣽⠟⠀",
	"⠀⠤⠄⠘⠛⢋⣖⣾⣿⣿⡟⢻⠿⠿⠿⠛⠁⠀⠀",
	"⠀⠀⠀⠀⣀⡘⢿⣿⣿⣿⣿⣏⠀⠀⠀⠀⠀⠀⠀",
	"⠀⠀⠀⠸⠿⠿⠷⠋⠙⠛⠻⢿⣷⠟⠀⠀⠀⠀⠀",
}

type helpSection struct {
	title string
	rows  [][]string
}

func (a *App) printHelp() {
	ui := spreadUI{color: shouldColorizeStdout()}
	a.printMenuHeader(ui, "", `szr or "sizer" is a token-aware CLI proxy built in Go`)

	a.printHelpSection(ui, helpSection{
		title: "Setup:",
		rows: [][]string{
			{"szr self install", "Install szr globally into ~/.local/bin or ~/bin."},
			{"szr uninstall", "Uninstall the global szr binary from this machine."},
			{"szr self update", "Update szr through Homebrew or go install when recognized."},
			{"szr self doctor [--json]", "Check PATH, config, cache, version, and optional tools."},
			{"szr install", "List available repo bootstrap targets."},
			{"szr install codex|claude-code|cursor|gemini|shell", "Install the target-specific szr bootstrap or hook integration."},
			{"szr install --all --print", "Preview all installer outputs without writing files."},
			{"szr uninstall codex|claude-code|cursor|gemini|shell", "Remove the target-specific szr bootstrap or hook integration."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Insight:",
		rows: [][]string{
			{"szr spread [--history|--json]", "Review savings, usage, hotspots, and fallback behavior."},
			{"szr recommend [--json]", "Turn command history into concrete tuning next steps."},
			{"szr hotspots [--json]", "Rank low-savings, fallback-heavy, and slow fingerprints."},
			{"szr explain <cmd...>", "Show the matched profile, budget, and rewrite decisions."},
			{"szr tee --latest", "Inspect the latest preserved full failure log."},
			{"szr profiles", "List builtin reducer profiles."},
			{"szr doctor [--history|--json]", "Check runtime diagnostics and optional history health."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Discover:",
		rows: [][]string{
			{"szr commands", "Show the full agent and power-user command catalog."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Global Flags:",
		rows: [][]string{
			{"-u, --ultra-compact", "Push harder compression when you want the shortest answer."},
			{"-v, -vv, -vvv, --verbose", "Show reducer decisions and raw command context."},
			{"--reasoning-budget <standard|agent>", "Prefer either human readability or stable agent loops."},
			{"--reasoning-budget-mode <standard|agent>", "Alias for reasoning budget mode selection."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Examples:",
		rows: [][]string{
			{"szr git status", "Summarize repo state with lower token cost."},
			{"szr go test ./...", "Reduce noisy test output while keeping failures."},
			{"szr compare go test ./...", "Show raw-vs-reduced output from one execution."},
			{"szr replay 1779393416870120000_go_test", "Rerender a preserved failure artifact without rerunning it."},
			{"szr spread --history", "Inspect real savings across recent commands."},
			{"szr tee --latest", "Open the most recent preserved full log."},
		},
	})
}

func (a *App) printCommands() {
	ui := spreadUI{color: shouldColorizeStdout()}
	a.printMenuHeader(ui, "commands", "full command catalog for agents and power users")

	a.printHelpSection(ui, helpSection{
		title: "Execution:",
		rows: [][]string{
			{"szr git <args...>", "Run git through the token-aware wrapper."},
			{"szr go <args...>", "Run go commands through the token-aware wrapper."},
			{"szr run <cmd...>", "Run any command through the default reducer path."},
			{"szr test <cmd...>", "Bias output toward failures and test signal."},
			{"szr summary <cmd...>", "Force summary-style rendering for generic commands."},
			{"szr proxy <cmd...>", "Run raw output without reduction."},
			{"szr compare <cmd...>", "Run once and compare raw vs reduced output."},
			{"szr replay <tee-id|file>", "Rerender preserved raw output through a selected profile."},
			{"szr explain <cmd...>", "Show profile, budget, and rewrite selection."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Local Tools:",
		rows: [][]string{
			{"szr ls [path]", "Render a compact directory tree."},
			{"szr read <file...>", "Read files through the file-aware reducer."},
			{"szr grep <pattern> [path]", "Search with grouped, stable match summaries."},
			{"szr rg <pattern> [path]", "Run ripgrep with szr-aware normalization."},
			{"szr json <file>", "Render JSON structure as readable typed paths."},
			{"szr log [file]", "Fold repeated log lines from stdin or a file."},
			{"szr tee [--latest|<id>|--json]", "Inspect preserved full-output artifacts."},
			{"szr tee find <query>", "Search preserved tee artifacts by id, command, or profile."},
			{"szr tee prune [flags]", "Remove stale or missing tee artifacts and rewrite the index."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Insight:",
		rows: [][]string{
			{"szr spread [--history|--json]", "Summarize savings, hotspots, and fallback rates."},
			{"szr gain [--history|--json]", "Alias for spread."},
			{"szr recommend [--json]", "Emit concrete tuning recommendations from command history."},
			{"szr hotspots [--json]", "Rank the commands that most need reducer work."},
			{"szr profiles", "List builtin profiles and contracts."},
			{"szr doctor [--history|--json]", "Check runtime tools plus optional history diagnostics."},
			{"szr bench [fixture...]", "Run the built-in benchmark fixtures."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Rules And Scaffolds:",
		rows: [][]string{
			{"szr rules check [path]", "Validate a project rule file and report counts."},
			{"szr rules test <cmd...>", "Test project rules against a sample command."},
			{"szr scaffold profile <name>", "Generate a profile stub and fixture skeleton."},
		},
	})
	a.printHelpSection(ui, helpSection{
		title: "Install:",
		rows: [][]string{
			{"szr self install [--update-shell]", "Install szr globally and optionally update shell rc."},
			{"szr uninstall [--print]", "Uninstall the global szr binary."},
			{"szr self update", "Update the current szr install when the channel is recognized."},
			{"szr self doctor [--json]", "Inspect the global install target and PATH state."},
			{"szr install codex", "Patch AGENTS.md and install shared Codex guidance under ~/.codex."},
			{"szr install shell", "Generate shell integration for this repo."},
			{"szr install cursor", "Install Cursor preToolUse hook files under ~/.cursor."},
			{"szr install claude-code", "Install Claude Code hook files into ~/.claude."},
			{"szr install gemini", "Install Gemini BeforeTool hook files under ~/.gemini."},
			{"szr install --all --print", "Preview all repo bootstrap outputs."},
			{"szr uninstall codex", "Remove Codex AGENTS.md patch and ~/.codex guidance."},
			{"szr uninstall claude-code", "Remove ~/.claude Claude Code hook files and settings patch."},
			{"szr uninstall --all --print", "Preview repo bootstrap removals without writing files."},
		},
	})
}

func (a *App) printHelpSection(ui spreadUI, section helpSection) {
	ui.section(section.title)
	ui.table(
		[]string{"command", "what it does"},
		section.rows,
		tableSpec{
			maxWidth: map[int]int{
				0: 42,
				1: 72,
			},
		},
	)
}

func (a *App) printMenuHeader(ui spreadUI, label, subtitle string) {
	for _, line := range compactBanner {
		a.printCenteredLine(line, ui.color, true, false)
	}
	if label != "" {
		a.printCenteredLine(label, ui.color, true, false)
	}
	if subtitle != "" {
		a.printCenteredLine(subtitle, ui.color, false, true)
	}
	fmt.Println()
}

func (a *App) printCenteredLine(text string, color, bold, dim bool) {
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	padding := 0
	width := a.menuHeaderWidth()
	textWidth := utf8.RuneCountInString(text)
	if textWidth < width {
		padding = (width - textWidth) / 2
	}
	if color {
		prefix := ""
		if bold {
			prefix += ansiBold
		}
		if dim {
			prefix += ansiDim
		}
		text = prefix + ansiSkyBlue + text + ansiReset
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", padding), text)
}

func (a *App) menuHeaderWidth() int {
	width := utf8.RuneCountInString(`szr or "sizer" is a token-aware CLI proxy built in Go`)
	for _, line := range compactBanner {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if lineWidth := utf8.RuneCountInString(line); lineWidth > width {
			width = lineWidth
		}
	}
	return width
}
