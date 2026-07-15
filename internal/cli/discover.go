package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/devr-tools/szr/internal/discover"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
)

const (
	discoverMinRatioSamples = 3
	discoverMaxRatio        = 0.95
)

// runDiscover scans local AI-agent session transcripts for Bash commands
// that ran without szr and estimates the savings szr would have captured.
// Read-only and local-only: transcripts are never modified or transmitted.
func (a *App) runDiscover(args []string) int {
	opts, asJSON, code := parseDiscoverOptions(args)
	if code != 0 {
		return code
	}
	if opts.Root == "" {
		fmt.Fprintln(os.Stderr, "szr: cannot locate home directory for agent transcripts")
		return 1
	}
	opts.Matcher = a.discoverMatcher()
	opts.Ratio = discoverRatios(a.history)
	return writeDiscoverOutput(discover.Scan(opts), asJSON)
}

func writeDiscoverOutput(report discover.Report, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return 0
	}
	if report.Files == 0 {
		fmt.Println("no agent transcripts found")
		return 0
	}
	renderDiscoverReport(report)
	return 0
}

type discoverArgState struct {
	opts   discover.Options
	asJSON bool
	all    bool
}

func parseDiscoverOptions(args []string) (discover.Options, bool, int) {
	state := discoverArgState{opts: discover.Options{Root: defaultDiscoverRoot()}}
	for i := 0; i < len(args); i++ {
		next, ok := parseDiscoverFlag(&state, args, i)
		if !ok {
			return state.opts, false, 2
		}
		i = next
	}
	if !state.all {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to resolve working directory: %v\n", err)
			return state.opts, false, 1
		}
		state.opts.Project = discover.EncodeProjectDir(cwd)
	}
	return state.opts, state.asJSON, 0
}

func parseDiscoverFlag(state *discoverArgState, args []string, index int) (int, bool) {
	switch args[index] {
	case "--json":
		state.asJSON = true
		return index, true
	case "--all":
		state.all = true
		return index, true
	case "--since":
		value, next, ok := discoverIntFlag(args, index, "--since")
		state.opts.Since = time.Duration(value) * 24 * time.Hour
		return next, ok
	case "--top":
		value, next, ok := discoverIntFlag(args, index, "--top")
		state.opts.Top = value
		return next, ok
	case "--root":
		return parseDiscoverRoot(state, args, index)
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown discover flag %s\n", args[index])
		return index, false
	}
}

func parseDiscoverRoot(state *discoverArgState, args []string, index int) (int, bool) {
	if index+1 >= len(args) {
		fmt.Fprintln(os.Stderr, "szr: discover requires a value after --root")
		return index, false
	}
	state.opts.Root = args[index+1]
	return index + 1, true
}

func discoverIntFlag(args []string, index int, flag string) (int, int, bool) {
	if index+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "szr: discover requires a value after %s\n", flag)
		return 0, index, false
	}
	value, err := strconv.Atoi(args[index+1])
	if err != nil || value <= 0 {
		fmt.Fprintf(os.Stderr, "szr: invalid discover %s value %q\n", flag, args[index+1])
		return 0, index, false
	}
	return value, index + 1, true
}

func defaultDiscoverRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// discoverMatcher routes a raw transcript command through the same engine
// matching szr explain uses: the sh -c wrapper reuses the existing shell
// and transparent-prefix unwrapping before profile matching.
func (a *App) discoverMatcher() discover.Matcher {
	return func(command string) (string, bool) {
		inv := engine.Invocation{
			Command: []string{"sh", "-c", command},
			Display: []string{"sh", "-c", command},
		}
		profile := a.engine.Explain(inv)
		return profile.Name, profile.Source != engine.SourceFallback
	}
}

type discoverProfileTally struct {
	count int
	raw   int
	saved int
}

// discoverRatios derives per-profile savings ratios from the user's own szr
// history and falls back to a conservative default for unseen profiles.
func discoverRatios(store *history.Store) discover.RatioFunc {
	tallies := loadDiscoverTallies(store)
	return func(profile string) float64 {
		return discoverRatioFor(tallies[profile])
	}
}

func loadDiscoverTallies(store *history.Store) map[string]*discoverProfileTally {
	tallies := map[string]*discoverProfileTally{}
	if store == nil {
		return tallies
	}
	records, err := store.LoadAll()
	if err != nil {
		return tallies
	}
	for _, record := range records {
		tallyDiscoverRecord(tallies, record)
	}
	return tallies
}

func tallyDiscoverRecord(tallies map[string]*discoverProfileTally, record history.Record) {
	if record.Passthrough || record.RawTokens <= 0 || record.Profile == "" {
		return
	}
	tally := tallies[record.Profile]
	if tally == nil {
		tally = &discoverProfileTally{}
		tallies[record.Profile] = tally
	}
	tally.count++
	tally.raw += record.RawTokens
	tally.saved += record.SavedTokens
}

func discoverRatioFor(tally *discoverProfileTally) float64 {
	if tally == nil || tally.count < discoverMinRatioSamples || tally.raw <= 0 {
		return discover.DefaultRatio
	}
	ratio := float64(tally.saved) / float64(tally.raw)
	switch {
	case ratio < 0:
		return 0
	case ratio > discoverMaxRatio:
		return discoverMaxRatio
	default:
		return ratio
	}
}

func renderDiscoverReport(report discover.Report) {
	ui := spreadUI{color: shouldColorizeStdout()}
	pct := percentSaved(report.MissedTokens, report.RawTokens)
	missedDisplay := fmt.Sprintf("%s (%.1f%%)", formatCompactCount(report.MissedTokens), pct)
	ui.header("Discover Summary")
	ui.alignedMetrics([][2]string{
		{"Projects scanned", fmt.Sprintf("%d", report.Projects)},
		{"Transcript files", fmt.Sprintf("%d", report.Files)},
		{"Bash commands seen", fmt.Sprintf("%d", report.BashCommands)},
		{"Unwrapped commands", fmt.Sprintf("%d", report.Unwrapped)},
		{"szr-wrapped (skipped)", fmt.Sprintf("%d", report.SkippedWrapped)},
		{"small output (skipped)", fmt.Sprintf("%d", report.SkippedTrivial)},
		{"Raw output tokens", formatCompactCount(report.RawTokens)},
		{"Est. missed savings", withBar(pct, missedDisplay, ui.color, true)},
	})
	renderDiscoverTop(ui, report.Top)
}

func renderDiscoverTop(ui spreadUI, stats []discover.CommandStat) {
	if len(stats) == 0 {
		return
	}
	ui.section("top unwrapped commands:")
	rows := make([][]string, 0, len(stats))
	for _, stat := range stats {
		rows = append(rows, discoverTopRow(stat))
	}
	ui.table(
		[]string{"command", "profile", "count", "raw", "missed", "est savings"},
		rows,
		tableSpec{
			alignRight: map[int]bool{2: true, 3: true, 4: true},
			maxWidth:   map[int]int{0: 40, 1: 18, 5: 20},
		},
	)
	top := stats[0]
	fmt.Printf("  top action: run %q through szr (profile %s)\n", top.Command, top.Profile)
}

func discoverTopRow(stat discover.CommandStat) []string {
	return []string{
		stat.Command,
		stat.Profile,
		fmt.Sprintf("%d", stat.Count),
		fmt.Sprintf("%d", stat.RawTokens),
		fmt.Sprintf("%d", stat.MissedTokens),
		fmt.Sprintf("%.0f%% %s", stat.Ratio*100, progressBar(stat.Ratio*100, 10, false, true)),
	}
}
