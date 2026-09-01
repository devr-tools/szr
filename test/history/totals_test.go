package history_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/history"
)

func totalsRecord(i int) history.Record {
	rec := history.Record{
		Timestamp:      time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
		Command:        compactionCommand(i),
		Profile:        "git-status",
		DurationMS:     int64(10 + i%50),
		RawTokens:      100 + i,
		FilteredTokens: 20,
		SavedTokens:    80 + i,
		SavingsPct:     float64(70 + i%20),
	}
	switch i % 7 {
	case 0:
		rec.ExitCode = 1
	case 1:
		rec.FallbackUsed = true
	case 2:
		rec.EmptyResult = true
	case 3:
		rec.TeePath = "/tmp/tee.log"
	case 4:
		rec.Passthrough = true
	}
	return rec
}

// The point of archiving is that compaction stops changing the reported
// numbers: totals plus the surviving window must equal what the full history
// would have reported.
func TestArchivedTotalsPreserveLifetimeSummary(t *testing.T) {
	const total = 120
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 8*1024, 10)

	all := make([]history.Record, 0, total)
	for i := 0; i < total; i++ {
		rec := totalsRecord(i)
		all = append(all, rec)
		if err := store.Append(rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	records, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load all: %v", err)
	}
	if len(records) >= total {
		t.Fatalf("expected compaction to have trimmed history, got %d of %d", len(records), total)
	}

	totals := store.Totals()
	if totals.Commands == 0 {
		t.Fatal("expected compaction to archive the records it removed")
	}
	if totals.Version != history.TotalsVersion {
		t.Fatalf("expected the sidecar to carry a schema version, got %d", totals.Version)
	}

	want := history.Summarize(all, 8)
	got := totals.Combine(history.Summarize(records, 8))

	if got.Commands != want.Commands {
		t.Fatalf("commands: got %d want %d", got.Commands, want.Commands)
	}
	for _, check := range []struct {
		name      string
		got, want int
	}{
		{"raw tokens", got.RawTokens, want.RawTokens},
		{"filtered tokens", got.FilteredTokens, want.FilteredTokens},
		{"saved tokens", got.SavedTokens, want.SavedTokens},
		{"raw bytes read", got.RawBytesRead, want.RawBytesRead},
		{"bytes parsed", got.BytesParsed, want.BytesParsed},
		{"bytes emitted", got.BytesEmitted, want.BytesEmitted},
		{"failures", got.Failures, want.Failures},
		{"fallbacks", got.Fallbacks, want.Fallbacks},
		{"empty results", got.EmptyResults, want.EmptyResults},
		{"tee count", got.TeeCount, want.TeeCount},
		{"passthrough commands", got.PassthroughCommands, want.PassthroughCommands},
		{"passthrough tokens", got.PassthroughTokens, want.PassthroughTokens},
	} {
		if check.got != check.want {
			t.Fatalf("%s: got %d want %d", check.name, check.got, check.want)
		}
	}
	if got.TotalDurationMS != want.TotalDurationMS {
		t.Fatalf("total duration: got %d want %d", got.TotalDurationMS, want.TotalDurationMS)
	}
	for _, check := range []struct {
		name      string
		got, want float64
	}{
		{"average pct", got.AveragePct, want.AveragePct},
		{"filtered savings pct", got.FilteredSavingsPct, want.FilteredSavingsPct},
		{"failure rate", got.FailureRate, want.FailureRate},
		{"fallback rate", got.FallbackRate, want.FallbackRate},
		{"empty result rate", got.EmptyResultRate, want.EmptyResultRate},
		{"tee rate", got.TeeRate, want.TeeRate},
	} {
		if !closeEnough(check.got, check.want, 0.01) {
			t.Fatalf("%s: got %.4f want %.4f", check.name, check.got, check.want)
		}
	}
	if got.ArchivedCommands != totals.Commands {
		t.Fatalf("expected the summary to report %d archived runs, got %d", totals.Commands, got.ArchivedCommands)
	}
}

// Percentiles and per-command tables cannot be rebuilt from sums, so they must
// keep describing the records still on disk rather than silently claiming to
// cover the archive.
func TestCombineLeavesWindowScopedFieldsUntouched(t *testing.T) {
	window := history.Summarize([]history.Record{totalsRecord(1), totalsRecord(2)}, 8)
	totals := history.Totals{Version: history.TotalsVersion, Commands: 500, RawTokens: 5_000, SavedTokens: 4_000}

	got := totals.Combine(window)
	if got.DurationP50MS != window.DurationP50MS || got.DurationP95MS != window.DurationP95MS {
		t.Fatal("expected duration percentiles to stay window-scoped")
	}
	if len(got.TopCommands) != len(window.TopCommands) {
		t.Fatalf("expected top commands to stay window-scoped, got %d", len(got.TopCommands))
	}
	if len(got.Recent) != len(window.Recent) {
		t.Fatalf("expected the recent list to stay window-scoped, got %d", len(got.Recent))
	}
	if got.Commands != window.Commands+500 || got.SavedTokens != window.SavedTokens+4_000 {
		t.Fatalf("expected additive totals to include the archive, got %#v", got)
	}
}

func TestCombineWithEmptyTotalsIsIdentity(t *testing.T) {
	window := history.Summarize([]history.Record{totalsRecord(1)}, 8)
	if got := (history.Totals{}).Combine(window); got.Commands != window.Commands || got.ArchivedCommands != 0 {
		t.Fatalf("expected empty totals to leave the summary alone, got %#v", got)
	}
}

func TestArchivingSkipsCommandsExcludedFromSavings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 512, 1)

	uninstall := totalsRecord(0)
	uninstall.Command = "szr uninstall"
	uninstall.RawTokens = 1_000_000
	if err := store.Append(uninstall); err != nil {
		t.Fatalf("append uninstall: %v", err)
	}
	for i := 1; i < 12; i++ {
		if err := store.Append(totalsRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	totals := store.Totals()
	if totals.RawTokens >= 1_000_000 {
		t.Fatalf("expected the excluded uninstall run to stay out of totals, got %d raw tokens", totals.RawTokens)
	}
}

func TestClearDiscardsArchivedTotals(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 8*1024, 10)
	for i := 0; i < 60; i++ {
		if err := store.Append(totalsRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if store.Totals().Commands == 0 {
		t.Fatal("expected records to be archived before clearing")
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if totals := store.Totals(); !totals.Empty() {
		t.Fatalf("expected cleared totals, got %#v", totals)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "history-totals.json")); !os.IsNotExist(err) {
		t.Fatalf("expected the totals sidecar to be removed, err=%v", err)
	}
}

func TestTotalsIgnoresUnreadableSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.jsonl")
	if err := os.WriteFile(filepath.Join(dir, "history-totals.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("seed sidecar: %v", err)
	}
	if totals := history.New(path).Totals(); !totals.Empty() {
		t.Fatalf("expected a malformed sidecar to read as empty, got %#v", totals)
	}
}

func TestTotalsSidecarIsValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := history.NewWithLimits(path, 8*1024, 10)
	for i := 0; i < 60; i++ {
		if err := store.Append(totalsRecord(i)); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), "history-totals.json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var decoded history.Totals
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("sidecar is not valid json: %v", err)
	}
	if decoded.FirstArchivedAt.IsZero() || decoded.LastArchivedAt.IsZero() {
		t.Fatalf("expected archive timestamps, got %#v", decoded)
	}
}
