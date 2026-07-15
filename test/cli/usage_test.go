package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/discover"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func usageTranscriptLine(t *testing.T, msgID, ts, cwd string, input, cacheCreate, cacheRead, output int) string {
	t.Helper()
	return discoverFixtureLine(t, map[string]any{
		"type":      "assistant",
		"timestamp": ts,
		"cwd":       cwd,
		"message": map[string]any{
			"id": msgID,
			"usage": map[string]any{
				"input_tokens":                input,
				"cache_creation_input_tokens": cacheCreate,
				"cache_read_input_tokens":     cacheRead,
				"output_tokens":               output,
			},
		},
	})
}

type usageSessionJSON struct {
	SessionID        string  `json:"session_id"`
	Cwd              string  `json:"cwd"`
	Turns            int     `json:"turns"`
	FreshInputTokens int     `json:"fresh_input_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens"`
	OutputTokens     int     `json:"output_tokens"`
	SZRCommands      int     `json:"szr_commands"`
	Emitted          int     `json:"szr_emitted_tokens_est"`
	Avoided          int     `json:"szr_avoided_tokens_est"`
	Ambiguous        int     `json:"ambiguous_records"`
	ScopeMatched     bool    `json:"session_scope_matched"`
	EmittedPct       float64 `json:"szr_emitted_pct_of_fresh_input"`
	AvoidedPct       float64 `json:"szr_avoided_pct_of_fresh_input"`
}

type usageReportJSON struct {
	Sessions []usageSessionJSON `json:"sessions"`
	Totals   struct {
		Sessions         int `json:"sessions"`
		FreshInputTokens int `json:"fresh_input_tokens"`
		SZRCommands      int `json:"szr_commands"`
		Emitted          int `json:"szr_emitted_tokens_est"`
		Avoided          int `json:"szr_avoided_tokens_est"`
		Ambiguous        int `json:"ambiguous_records"`
	} `json:"totals"`
	Notes []string `json:"notes"`
}

func decodeUsageJSON(t *testing.T, stdout string) usageReportJSON {
	t.Helper()
	var report usageReportJSON
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode usage json: %v (stdout %q)", err, stdout)
	}
	return report
}

func TestUsageCommandCorrelatesHistory(t *testing.T) {
	root := t.TempDir()
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "sess-a.jsonl"), []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj", 100, 900, 5000, 300),
		usageTranscriptLine(t, "msg_2", "2026-07-10T11:00:00Z", "/work/proj", 0, 0, 2000, 100),
	})

	records := []history.Record{
		// Correlated by cwd + time window.
		{Timestamp: time.Date(2026, 7, 10, 10, 30, 0, 0, time.UTC), Cwd: "/work/proj", FilteredTokens: 200, SavedTokens: 800},
		// Correlated exactly by session scope despite a foreign cwd.
		{Timestamp: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), Cwd: "/elsewhere", SessionScope: "sess-a", FilteredTokens: 50, SavedTokens: 150},
		// Matching time but wrong directory: never correlated.
		{Timestamp: time.Date(2026, 7, 10, 10, 30, 0, 0, time.UTC), Cwd: "/other", FilteredTokens: 999, SavedTokens: 999},
	}
	app := discoverApp(t, records)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	report := decodeUsageJSON(t, stdout)
	if len(report.Sessions) != 1 {
		t.Fatalf("expected one session, got %+v", report.Sessions)
	}
	session := report.Sessions[0]
	if session.SessionID != "sess-a" || session.Turns != 2 || session.FreshInputTokens != 1000 {
		t.Fatalf("unexpected model side: %+v", session)
	}
	if session.CacheReadTokens != 7000 || session.OutputTokens != 400 {
		t.Fatalf("unexpected cache/output sums: %+v", session)
	}
	if session.SZRCommands != 2 || session.Emitted != 250 || session.Avoided != 950 || !session.ScopeMatched {
		t.Fatalf("unexpected szr side: %+v", session)
	}
	if session.EmittedPct != 25 || session.AvoidedPct != 95 {
		t.Fatalf("unexpected percentages: %+v", session)
	}
	if report.Totals.SZRCommands != 2 || report.Totals.FreshInputTokens != 1000 {
		t.Fatalf("unexpected totals: %+v", report.Totals)
	}
	if len(report.Notes) < 3 || !strings.Contains(report.Notes[0], "heuristic estimates") {
		t.Fatalf("expected estimate note, got %+v", report.Notes)
	}
}

func TestUsageCommandFlagsAmbiguousWindowMatches(t *testing.T) {
	root := t.TempDir()
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "sess-old.jsonl"), []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj", 10, 0, 0, 1),
		usageTranscriptLine(t, "msg_2", "2026-07-10T12:00:00Z", "/work/proj", 10, 0, 0, 1),
	})
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "sess-new.jsonl"), []string{
		usageTranscriptLine(t, "msg_3", "2026-07-10T11:00:00Z", "/work/proj", 10, 0, 0, 1),
		usageTranscriptLine(t, "msg_4", "2026-07-10T13:00:00Z", "/work/proj", 10, 0, 0, 1),
	})

	records := []history.Record{
		// Inside both windows: parallel sessions in one directory.
		{Timestamp: time.Date(2026, 7, 10, 11, 30, 0, 0, time.UTC), Cwd: "/work/proj", FilteredTokens: 40, SavedTokens: 60},
	}
	app := discoverApp(t, records)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	report := decodeUsageJSON(t, stdout)
	if len(report.Sessions) != 2 {
		t.Fatalf("expected two sessions, got %+v", report.Sessions)
	}
	newest := report.Sessions[0]
	if newest.SessionID != "sess-new" || newest.SZRCommands != 1 || newest.Ambiguous != 1 {
		t.Fatalf("expected ambiguous record on newest session, got %+v", report.Sessions)
	}
	if report.Sessions[1].SZRCommands != 0 {
		t.Fatalf("expected no attribution to older session, got %+v", report.Sessions[1])
	}
	if report.Totals.Ambiguous != 1 {
		t.Fatalf("expected ambiguous total, got %+v", report.Totals)
	}
	found := false
	for _, note := range report.Notes {
		if strings.Contains(note, "multiple session windows") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ambiguity note, got %+v", report.Notes)
	}
}

func TestUsageCommandSessionFilterAndTable(t *testing.T) {
	root := t.TempDir()
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "aaa-1.jsonl"), []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj", 10, 0, 5, 1),
	})
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "bbb-2.jsonl"), []string{
		usageTranscriptLine(t, "msg_2", "2026-07-10T10:00:00Z", "/work/proj", 10, 0, 5, 1),
	})

	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--session", "aaa")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"Usage Summary",
		"sessions (newest first):",
		"aaa-1",
		"total",
		"note: szr token numbers are heuristic estimates",
		"note: records without a session scope are correlated by directory + session time window",
		"note: 'w/o szr' compares avoided tokens to fresh input only",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected usage output %q in %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "bbb-2") {
		t.Fatalf("expected session filter to drop bbb-2: %q", stdout)
	}
}

func TestUsageCommandScopesToCurrentProject(t *testing.T) {
	root := t.TempDir()
	workDir := t.TempDir()
	restore := testutil.Chdir(t, workDir)
	defer restore()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	writeDiscoverTranscript(t, filepath.Join(root, discover.EncodeProjectDir(cwd), "scoped.jsonl"), []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", workDir, 10, 0, 0, 1),
	})
	writeDiscoverTranscript(t, filepath.Join(root, "-other-proj", "foreign.jsonl"), []string{
		usageTranscriptLine(t, "msg_2", "2026-07-10T10:00:00Z", "/other", 10, 0, 0, 1),
	})

	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected scoped usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	report := decodeUsageJSON(t, stdout)
	if len(report.Sessions) != 1 || report.Sessions[0].SessionID != "scoped" {
		t.Fatalf("expected current-project scope, got %+v", report.Sessions)
	}
}

func TestUsageCommandEmptyAndBadFlags(t *testing.T) {
	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", filepath.Join(t.TempDir(), "none"), "--all")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "no agent sessions with model usage found") {
		t.Fatalf("unexpected empty output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, _, stderr = testutil.RunApp(t, app, "usage", "--bogus")
	if code != 2 || !strings.Contains(stderr, "unknown usage flag") {
		t.Fatalf("expected flag error, got code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = testutil.RunApp(t, app, "usage", "--since", "zero")
	if code != 2 || !strings.Contains(stderr, "invalid usage --since value") {
		t.Fatalf("expected since error, got code=%d stderr=%q", code, stderr)
	}
}
