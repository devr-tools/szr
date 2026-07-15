package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/devr-tools/szr/internal/discover"
	"github.com/devr-tools/szr/internal/history"
)

const (
	// usageMaxTableRows caps the rendered table; --json stays uncapped.
	usageMaxTableRows = 20
	// usageWindowSlack widens a session's time window when correlating szr
	// history records by cwd + time, absorbing clock and flush skew.
	usageWindowSlack = 2 * time.Minute
)

// runUsage reports, per agent session, the model-billed token usage recorded
// in local transcripts next to the szr-side estimates from command history.
// Read-only and local-only.
func (a *App) runUsage(args []string) int {
	state, code := parseUsageOptions(args)
	if code != 0 {
		return code
	}
	if state.opts.Root == "" {
		fmt.Fprintln(os.Stderr, "szr: cannot locate home directory for agent transcripts")
		return 1
	}
	sessions := discover.ScanUsage(state.opts)
	records := a.loadUsageRecords()
	report := buildUsageReport(sessions, records)
	return writeUsageOutput(report, state.asJSON)
}

func (a *App) loadUsageRecords() []history.Record {
	if a.history == nil {
		return nil
	}
	records, err := a.history.LoadAll()
	if err != nil {
		return nil
	}
	return records
}

type usageArgState struct {
	opts   discover.UsageOptions
	asJSON bool
	all    bool
}

func parseUsageOptions(args []string) (usageArgState, int) {
	state := usageArgState{opts: discover.UsageOptions{Root: defaultDiscoverRoot()}}
	for i := 0; i < len(args); i++ {
		next, ok := parseUsageFlag(&state, args, i)
		if !ok {
			return state, 2
		}
		i = next
	}
	if !state.all {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "szr: failed to resolve working directory: %v\n", err)
			return state, 1
		}
		state.opts.Project = discover.EncodeProjectDir(cwd)
	}
	return state, 0
}

func parseUsageFlag(state *usageArgState, args []string, index int) (int, bool) {
	switch args[index] {
	case "--json":
		state.asJSON = true
		return index, true
	case "--all":
		state.all = true
		return index, true
	case "--since":
		value, next, ok := usageIntFlag(args, index, "--since")
		state.opts.Since = time.Duration(value) * 24 * time.Hour
		return next, ok
	case "--session":
		return parseUsageStringFlag(&state.opts.SessionPrefix, args, index, "--session")
	case "--root":
		return parseUsageStringFlag(&state.opts.Root, args, index, "--root")
	default:
		fmt.Fprintf(os.Stderr, "szr: unknown usage flag %s\n", args[index])
		return index, false
	}
}

func parseUsageStringFlag(target *string, args []string, index int, flag string) (int, bool) {
	if index+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "szr: usage requires a value after %s\n", flag)
		return index, false
	}
	*target = args[index+1]
	return index + 1, true
}

func usageIntFlag(args []string, index int, flag string) (int, int, bool) {
	if index+1 >= len(args) {
		fmt.Fprintf(os.Stderr, "szr: usage requires a value after %s\n", flag)
		return 0, index, false
	}
	value, err := strconv.Atoi(args[index+1])
	if err != nil || value <= 0 {
		fmt.Fprintf(os.Stderr, "szr: invalid usage %s value %q\n", flag, args[index+1])
		return 0, index, false
	}
	return value, index + 1, true
}

type usageSessionRow struct {
	discover.SessionUsage
	FreshInputTokens int `json:"fresh_input_tokens"`
	SZRCommands      int `json:"szr_commands"`
	// Token counts below are szr-side heuristic estimates, not billed values.
	SZREmittedTokens int  `json:"szr_emitted_tokens_est"`
	SZRAvoidedTokens int  `json:"szr_avoided_tokens_est"`
	AmbiguousRecords int  `json:"ambiguous_records,omitempty"`
	ScopeMatched     bool `json:"session_scope_matched,omitempty"`
	// EmittedPct is szr-emitted tokens as a share of the model's fresh input;
	// AvoidedPct is how much larger fresh input would have been without szr.
	// Both exclude cache reads.
	EmittedPct float64 `json:"szr_emitted_pct_of_fresh_input"`
	AvoidedPct float64 `json:"szr_avoided_pct_of_fresh_input"`
}

type usageTotals struct {
	Sessions         int     `json:"sessions"`
	Turns            int     `json:"turns"`
	FreshInputTokens int     `json:"fresh_input_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	SZRCommands      int     `json:"szr_commands"`
	SZREmittedTokens int     `json:"szr_emitted_tokens_est"`
	SZRAvoidedTokens int     `json:"szr_avoided_tokens_est"`
	AmbiguousRecords int     `json:"ambiguous_records,omitempty"`
	EmittedPct       float64 `json:"szr_emitted_pct_of_fresh_input"`
	AvoidedPct       float64 `json:"szr_avoided_pct_of_fresh_input"`
}

type usageReport struct {
	Sessions []usageSessionRow `json:"sessions"`
	Totals   usageTotals       `json:"totals"`
	Notes    []string          `json:"notes"`
}

func buildUsageReport(sessions []discover.SessionUsage, records []history.Record) usageReport {
	rows := make([]usageSessionRow, len(sessions))
	for i, session := range sessions {
		rows[i] = usageSessionRow{SessionUsage: session, FreshInputTokens: session.FreshInputTokens()}
	}
	correlateUsageRecords(rows, records)
	for i := range rows {
		rows[i].EmittedPct = usagePct(rows[i].SZREmittedTokens, rows[i].FreshInputTokens)
		rows[i].AvoidedPct = usagePct(rows[i].SZRAvoidedTokens, rows[i].FreshInputTokens)
	}
	totals := sumUsageRows(rows)
	return usageReport{Sessions: rows, Totals: totals, Notes: usageNotes(totals)}
}

// correlateUsageRecords attributes each szr history record to one session: an
// exact match when the record carries a session scope equal to a session id,
// otherwise by working directory plus session time window. A record whose
// window matches several sessions (parallel sessions in one directory) goes
// to the newest-started one and is counted as ambiguous.
func correlateUsageRecords(rows []usageSessionRow, records []history.Record) {
	for _, record := range records {
		if row := usageScopeMatch(rows, record); row != nil {
			row.ScopeMatched = true
			addUsageRecord(row, record)
			continue
		}
		matches := usageWindowMatches(rows, record)
		if len(matches) == 0 {
			continue
		}
		row := newestStartedRow(rows, matches)
		if len(matches) > 1 {
			row.AmbiguousRecords++
		}
		addUsageRecord(row, record)
	}
}

func usageScopeMatch(rows []usageSessionRow, record history.Record) *usageSessionRow {
	if record.SessionScope == "" {
		return nil
	}
	for i := range rows {
		if rows[i].SessionID == record.SessionScope {
			return &rows[i]
		}
	}
	return nil
}

func usageWindowMatches(rows []usageSessionRow, record history.Record) []int {
	var matches []int
	for i, row := range rows {
		if row.Cwd == "" || row.Cwd != record.Cwd {
			continue
		}
		if record.Timestamp.Before(row.FirstSeen.Add(-usageWindowSlack)) {
			continue
		}
		if record.Timestamp.After(row.LastSeen.Add(usageWindowSlack)) {
			continue
		}
		matches = append(matches, i)
	}
	return matches
}

func newestStartedRow(rows []usageSessionRow, matches []int) *usageSessionRow {
	best := matches[0]
	for _, index := range matches[1:] {
		if rows[index].FirstSeen.After(rows[best].FirstSeen) {
			best = index
		}
	}
	return &rows[best]
}

func addUsageRecord(row *usageSessionRow, record history.Record) {
	row.SZRCommands++
	row.SZREmittedTokens += record.FilteredTokens
	row.SZRAvoidedTokens += record.SavedTokens
}

func sumUsageRows(rows []usageSessionRow) usageTotals {
	totals := usageTotals{Sessions: len(rows)}
	for _, row := range rows {
		totals.Turns += row.Turns
		totals.FreshInputTokens += row.FreshInputTokens
		totals.CacheReadTokens += row.CacheReadTokens
		totals.OutputTokens += row.OutputTokens
		totals.SZRCommands += row.SZRCommands
		totals.SZREmittedTokens += row.SZREmittedTokens
		totals.SZRAvoidedTokens += row.SZRAvoidedTokens
		totals.AmbiguousRecords += row.AmbiguousRecords
	}
	totals.EmittedPct = usagePct(totals.SZREmittedTokens, totals.FreshInputTokens)
	totals.AvoidedPct = usagePct(totals.SZRAvoidedTokens, totals.FreshInputTokens)
	return totals
}

func usagePct(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

func usageNotes(totals usageTotals) []string {
	notes := []string{
		"szr token numbers are heuristic estimates; model numbers are exact as recorded by the agent runtime",
		"records without a session scope are correlated by directory + session time window",
		"'w/o szr' compares avoided tokens to fresh input only; cache reads are excluded",
	}
	if totals.AmbiguousRecords > 0 {
		notes = append(notes, fmt.Sprintf("%d record(s) matched multiple session windows and were assigned to the newest session", totals.AmbiguousRecords))
	}
	return notes
}

func writeUsageOutput(report usageReport, asJSON bool) int {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return 0
	}
	if len(report.Sessions) == 0 {
		fmt.Println("no agent sessions with model usage found")
		return 0
	}
	renderUsageReport(report)
	return 0
}
