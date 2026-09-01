package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TotalsVersion is the schema version of the archived-totals sidecar.
const TotalsVersion = 1

// Totals carries the measurements of records compaction has removed from the
// history file, so lifetime reporting keeps counting runs that are no longer
// on disk. Without it, `szr spread` silently reports a shrinking window of
// savings every time compaction trims the file.
//
// Only additive counters belong here. Percentiles, per-command tables, and
// hotspot scores cannot be reconstructed from sums, so they stay scoped to the
// records still on disk and are reported as such rather than being folded in
// and quietly becoming wrong.
type Totals struct {
	Version             int   `json:"version"`
	Commands            int   `json:"commands"`
	RawTokens           int   `json:"raw_tokens"`
	FilteredTokens      int   `json:"filtered_tokens"`
	SavedTokens         int   `json:"saved_tokens"`
	PassthroughCommands int   `json:"passthrough_commands,omitempty"`
	PassthroughTokens   int   `json:"passthrough_tokens,omitempty"`
	TotalDurationMS     int64 `json:"total_duration_ms"`
	Failures            int   `json:"failures"`
	Fallbacks           int   `json:"fallbacks"`
	EmptyResults        int   `json:"empty_results"`
	TeeCount            int   `json:"tee_count"`
	RawBytesRead        int   `json:"raw_bytes_read"`
	BytesParsed         int   `json:"bytes_parsed"`
	BytesEmitted        int   `json:"bytes_emitted"`
	// SavingsPctSum sums the per-record savings percentages of archived
	// non-passthrough runs. Averages need the sum, not the average, to stay
	// correct when combined with the window.
	SavingsPctSum float64 `json:"savings_pct_sum"`
	// DroppedRecords counts lines compaction could not parse at all, so a
	// report can admit the gap instead of implying complete coverage.
	DroppedRecords  int       `json:"dropped_records,omitempty"`
	FirstArchivedAt time.Time `json:"first_archived_at,omitempty"`
	LastArchivedAt  time.Time `json:"last_archived_at,omitempty"`
}

// Empty reports totals that have never archived anything.
func (t Totals) Empty() bool {
	return t.Commands == 0 && t.DroppedRecords == 0
}

// totalsPath is the sidecar next to the history file: history.jsonl pairs with
// history-totals.json.
func (s *Store) totalsPath() string {
	base := filepath.Base(s.path)
	return filepath.Join(filepath.Dir(s.path), strings.TrimSuffix(base, ".jsonl")+"-totals.json")
}

// Totals returns the archived counters. A missing or unreadable sidecar yields
// zero totals rather than an error: it is advisory reporting state, and a
// report that starts from zero is better than a command that refuses to run.
func (s *Store) Totals() Totals {
	data, err := os.ReadFile(s.totalsPath())
	if err != nil {
		return Totals{}
	}
	var totals Totals
	if err := json.Unmarshal(data, &totals); err != nil {
		return Totals{}
	}
	return totals
}

// archive folds records compaction is about to delete into the sidecar. It is
// called only after the compacted file is in place: a crash between the two
// loses this batch of counters, which is strictly better than counting the
// same runs twice and permanently inflating reported savings.
func (s *Store) archive(records []Record, dropped int, now time.Time) {
	if len(records) == 0 && dropped == 0 {
		return
	}
	totals := s.Totals()
	totals.Version = TotalsVersion
	totals.DroppedRecords += dropped
	foldRecords(&totals, records)
	if totals.FirstArchivedAt.IsZero() {
		totals.FirstArchivedAt = now
	}
	totals.LastArchivedAt = now
	s.writeTotals(totals)
}

// foldRecords adds records to the archived counters, skipping the commands
// savings reporting excludes so the archive and the window agree.
func foldRecords(totals *Totals, records []Record) {
	for _, raw := range records {
		rec := hydrateRecord(raw)
		if SavingsExcludedCommand(rec.Command) {
			continue
		}
		foldRecordVolume(totals, rec)
		foldRecordOutcome(totals, rec)
	}
}

func foldRecordVolume(totals *Totals, rec Record) {
	totals.Commands++
	totals.RawTokens += rec.RawTokens
	totals.FilteredTokens += rec.FilteredTokens
	totals.SavedTokens += rec.SavedTokens
	totals.TotalDurationMS += rec.DurationMS
	totals.RawBytesRead += rec.RawBytesRead
	totals.BytesParsed += rec.BytesParsed
	totals.BytesEmitted += rec.BytesEmitted
}

func foldRecordOutcome(totals *Totals, rec Record) {
	if rec.Passthrough {
		totals.PassthroughCommands++
		totals.PassthroughTokens += rec.RawTokens
	} else {
		totals.SavingsPctSum += rec.SavingsPct
	}
	if rec.ExitCode != 0 {
		totals.Failures++
	}
	if rec.FallbackUsed {
		totals.Fallbacks++
	}
	if rec.EmptyResult {
		totals.EmptyResults++
	}
	if rec.TeePath != "" {
		totals.TeeCount++
	}
}

// writeTotals replaces the sidecar atomically so a crash mid-write cannot
// leave a truncated file behind.
func (s *Store) writeTotals(totals Totals) {
	data, err := json.Marshal(totals)
	if err != nil {
		return
	}
	path := s.totalsPath()
	tmp, err := os.CreateTemp(filepath.Dir(path), "history-totals-*.tmp")
	if err != nil {
		return
	}
	if !writeTotalsTemp(tmp, append(data, '\n')) {
		_ = os.Remove(tmp.Name())
		return
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
	}
}

func writeTotalsTemp(tmp *os.File, data []byte) bool {
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return false
	}
	return tmp.Close() == nil
}

// Combine returns summary with the archived counters folded into its additive
// totals and every derived rate recomputed from the combined numbers.
//
// Window-scoped fields are left untouched: the duration percentiles, top
// commands, profile table, hotspots, budget suggestions, and recent list all
// describe the records still on disk. Summary.ArchivedCommands tells a reader
// how many runs the totals include beyond them.
func (t Totals) Combine(summary Summary) Summary {
	if t.Empty() {
		return summary
	}

	// The window summary carries an average, so recover its sum before the
	// combined counts change the denominator.
	savingsPctSum := summary.AveragePct*float64(summary.Commands-summary.PassthroughCommands) + t.SavingsPctSum
	summary.ArchivedCommands = t.Commands
	summary.ArchivedDroppedRecords = t.DroppedRecords
	addArchivedCounters(&summary, t)
	recomputeSummaryRates(&summary, savingsPctSum)
	return summary
}

func addArchivedCounters(summary *Summary, t Totals) {
	summary.Commands += t.Commands
	summary.RawTokens += t.RawTokens
	summary.FilteredTokens += t.FilteredTokens
	summary.SavedTokens += t.SavedTokens
	summary.TotalDurationMS += t.TotalDurationMS
	summary.RawBytesRead += t.RawBytesRead
	summary.BytesParsed += t.BytesParsed
	summary.BytesEmitted += t.BytesEmitted
	summary.PassthroughCommands += t.PassthroughCommands
	summary.PassthroughTokens += t.PassthroughTokens
	summary.Failures += t.Failures
	summary.Fallbacks += t.Fallbacks
	summary.EmptyResults += t.EmptyResults
	summary.TeeCount += t.TeeCount
}

func recomputeSummaryRates(summary *Summary, savingsPctSum float64) {
	if samples := summary.Commands - summary.PassthroughCommands; samples > 0 {
		summary.AveragePct = savingsPctSum / float64(samples)
	}
	summary.FilteredSavingsPct = percent(summary.SavedTokens, summary.RawTokens-summary.PassthroughTokens)
	summary.FailureRate = percent(summary.Failures, summary.Commands)
	summary.FallbackRate = percent(summary.Fallbacks, summary.Commands)
	summary.EmptyResultRate = percent(summary.EmptyResults, summary.Commands)
	summary.TeeRate = percent(summary.TeeCount, summary.Commands)
}
