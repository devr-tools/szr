// Package discover scans locally stored AI-agent session transcripts for
// shell commands that ran outside szr and estimates the token savings szr
// would have captured. It never modifies transcript files and never sends
// data anywhere.
package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

const (
	DefaultSinceDays = 30
	DefaultTop       = 15
	DefaultRatio     = 0.60
	minOutputBytes   = 200
)

// Matcher reports the profile szr's routing would select for a raw command
// string, and whether a non-fallback profile matched.
type Matcher func(command string) (profile string, matched bool)

// RatioFunc returns the expected savings ratio (0..1) for a profile.
type RatioFunc func(profile string) float64

type Options struct {
	// Root is the transcripts root, e.g. ~/.claude/projects.
	Root string
	// Project restricts the scan to one encoded project dir; empty scans all.
	Project string
	Since   time.Duration
	Now     time.Time
	Top     int
	Matcher Matcher
	Ratio   RatioFunc
}

type Report struct {
	Projects       int           `json:"projects"`
	Files          int           `json:"files"`
	BashCommands   int           `json:"bash_commands"`
	Unwrapped      int           `json:"unwrapped_commands"`
	SkippedWrapped int           `json:"szr_wrapped_skipped"`
	SkippedTrivial int           `json:"trivial_skipped"`
	RawTokens      int           `json:"raw_tokens"`
	MissedTokens   int           `json:"estimated_missed_tokens"`
	Top            []CommandStat `json:"top"`
}

type CommandStat struct {
	Command      string  `json:"command"`
	Profile      string  `json:"profile"`
	Matched      bool    `json:"matched"`
	Count        int     `json:"count"`
	RawTokens    int     `json:"raw_tokens"`
	MissedTokens int     `json:"estimated_missed_tokens"`
	Ratio        float64 `json:"savings_ratio"`
}

type commandGroup struct {
	count     int
	rawTokens int
}

// EncodeProjectDir converts an absolute project path into the transcript
// directory name used under Root: every non-alphanumeric byte becomes '-'.
func EncodeProjectDir(path string) string {
	var out strings.Builder
	out.Grow(len(path))
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			continue
		}
		out.WriteByte('-')
	}
	return out.String()
}

func Scan(opts Options) Report {
	opts = withDefaults(opts)
	report := Report{}
	groups := map[string]*commandGroup{}
	cutoff := opts.Now.Add(-opts.Since)
	for _, dir := range projectDirs(opts) {
		files := transcriptFiles(dir, cutoff)
		if len(files) == 0 {
			continue
		}
		report.Projects++
		report.Files += len(files)
		for _, file := range files {
			report.BashCommands += scanTranscriptFile(file, func(run commandRun) {
				recordRun(&report, groups, run)
			})
		}
	}
	report.Top, report.RawTokens, report.MissedTokens = finalizeGroups(groups, opts)
	return report
}

func withDefaults(opts Options) Options {
	if opts.Since <= 0 {
		opts.Since = DefaultSinceDays * 24 * time.Hour
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.Top <= 0 {
		opts.Top = DefaultTop
	}
	if opts.Ratio == nil {
		opts.Ratio = func(string) float64 { return DefaultRatio }
	}
	if opts.Matcher == nil {
		opts.Matcher = func(string) (string, bool) { return "", false }
	}
	return opts
}

func recordRun(report *Report, groups map[string]*commandGroup, run commandRun) {
	switch {
	case isSZRWrapped(run.Command):
		report.SkippedWrapped++
	case len(run.Output) < minOutputBytes:
		report.SkippedTrivial++
	default:
		report.Unwrapped++
		group, ok := groups[run.Command]
		if !ok {
			group = &commandGroup{}
			groups[run.Command] = group
		}
		group.count++
		group.rawTokens += history.EstimateTokens(run.Output)
	}
}

func isSZRWrapped(command string) bool {
	trimmed := strings.TrimSpace(command)
	return trimmed == "szr" || strings.HasPrefix(trimmed, "szr ") || strings.Contains(command, " szr ")
}

func finalizeGroups(groups map[string]*commandGroup, opts Options) ([]CommandStat, int, int) {
	stats := make([]CommandStat, 0, len(groups))
	rawTotal := 0
	missedTotal := 0
	for command, group := range groups {
		stat := buildCommandStat(command, group, opts)
		rawTotal += stat.RawTokens
		missedTotal += stat.MissedTokens
		stats = append(stats, stat)
	}
	sortCommandStats(stats)
	if len(stats) > opts.Top {
		stats = stats[:opts.Top]
	}
	return stats, rawTotal, missedTotal
}

func buildCommandStat(command string, group *commandGroup, opts Options) CommandStat {
	profile, matched := opts.Matcher(command)
	ratio := opts.Ratio(profile)
	return CommandStat{
		Command:      command,
		Profile:      profile,
		Matched:      matched,
		Count:        group.count,
		RawTokens:    group.rawTokens,
		MissedTokens: estimateMissed(group.rawTokens, ratio),
		Ratio:        ratio,
	}
}

func estimateMissed(rawTokens int, ratio float64) int {
	if rawTokens <= 0 || ratio <= 0 {
		return 0
	}
	return int(float64(rawTokens)*ratio + 0.5)
}

func sortCommandStats(stats []CommandStat) {
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].MissedTokens != stats[j].MissedTokens {
			return stats[i].MissedTokens > stats[j].MissedTokens
		}
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Command < stats[j].Command
	})
}

func projectDirs(opts Options) []string {
	if opts.Project != "" {
		dir := filepath.Join(opts.Root, opts.Project)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return []string{dir}
		}
		return nil
	}
	entries, err := os.ReadDir(opts.Root)
	if err != nil {
		return nil
	}
	dirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(opts.Root, entry.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs
}
