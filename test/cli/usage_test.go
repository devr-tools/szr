package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

type usageAgentJSON struct {
	AgentID             string `json:"agent_id"`
	Turns               int    `json:"turns"`
	InputTokens         int    `json:"input_tokens"`
	CacheCreationTokens int    `json:"cache_creation_tokens"`
	FreshInputTokens    int    `json:"fresh_input_tokens"`
	CacheReadTokens     int    `json:"cache_read_tokens"`
	OutputTokens        int    `json:"output_tokens"`
}

type usageSessionJSON struct {
	SessionID        string           `json:"session_id"`
	Cwd              string           `json:"cwd"`
	Turns            int              `json:"turns"`
	FreshInputTokens int              `json:"fresh_input_tokens"`
	CacheReadTokens  int              `json:"cache_read_tokens"`
	OutputTokens     int              `json:"output_tokens"`
	AgentCount       int              `json:"agent_count"`
	Agents           []usageAgentJSON `json:"agents"`
	Main             usageAgentJSON   `json:"main"`
	SZRCommands      int              `json:"szr_commands"`
	Emitted          int              `json:"szr_emitted_tokens_est"`
	Avoided          int              `json:"szr_avoided_tokens_est"`
	Ambiguous        int              `json:"ambiguous_records"`
	ScopeMatched     bool             `json:"session_scope_matched"`
	EmittedPct       float64          `json:"szr_emitted_pct_of_fresh_input"`
	AvoidedPct       float64          `json:"szr_avoided_pct_of_fresh_input"`
}

type usageReportJSON struct {
	Sessions []usageSessionJSON `json:"sessions"`
	Totals   struct {
		Sessions         int `json:"sessions"`
		AgentCount       int `json:"agent_count"`
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

func writeUsageAgentFixture(t *testing.T, root string) {
	t.Helper()
	session := filepath.Join(root, "-work-proj", "sess-agents")
	writeDiscoverTranscript(t, session+".jsonl", []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj", 100, 900, 5000, 300),
	})
	writeDiscoverTranscript(t, filepath.Join(session, "subagents", "agent-a1b2c3d4e5f6.jsonl"), []string{
		usageTranscriptLine(t, "msg_2", "2026-07-10T10:05:00Z", "/work/proj", 10, 90, 500, 30),
	})
	writeDiscoverTranscript(t, filepath.Join(session, "subagents", "agent-f6e5d4c3b2a1.jsonl"), []string{
		usageTranscriptLine(t, "msg_3", "2026-07-10T10:06:00Z", "/work/proj", 5, 45, 250, 15),
	})
}

func TestUsageCommandAgentBreakdownJSON(t *testing.T) {
	root := t.TempDir()
	writeUsageAgentFixture(t, root)
	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	report := decodeUsageJSON(t, stdout)
	if len(report.Sessions) != 1 {
		t.Fatalf("expected one session, got %+v", report.Sessions)
	}
	session := report.Sessions[0]
	if session.AgentCount != 2 || len(session.Agents) != 2 {
		t.Fatalf("unexpected agent count: %+v", session)
	}
	first := session.Agents[0]
	if first.AgentID != "a1b2c3d4e5f6" || first.Turns != 1 || first.FreshInputTokens != 100 ||
		first.CacheReadTokens != 500 || first.OutputTokens != 30 {
		t.Fatalf("unexpected first agent: %+v", first)
	}
	second := session.Agents[1]
	if second.AgentID != "f6e5d4c3b2a1" || second.FreshInputTokens != 50 || second.OutputTokens != 15 {
		t.Fatalf("unexpected second agent: %+v", second)
	}
	if session.Main.AgentID != "" || session.Main.Turns != 1 ||
		session.Main.InputTokens != 100 || session.Main.CacheCreationTokens != 900 || session.Main.OutputTokens != 300 {
		t.Fatalf("unexpected main usage: %+v", session.Main)
	}
	if session.Turns != 3 || session.FreshInputTokens != 1150 || session.CacheReadTokens != 5750 || session.OutputTokens != 345 {
		t.Fatalf("expected totals to equal main plus agents: %+v", session)
	}
	if report.Totals.AgentCount != 2 {
		t.Fatalf("unexpected totals agent count: %+v", report.Totals)
	}
}

func TestUsageCommandAgentDrillDownTable(t *testing.T) {
	root := t.TempDir()
	writeUsageAgentFixture(t, root)
	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--session", "sess-agents")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"agents",
		"agents in session sess-age:",
		"a1b2c3d4",
		"f6e5d4c3",
		"main",
		"total",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected drill-down output %q in %q", want, stdout)
		}
	}
	// turns column (3) followed by the agents column (2) in the session row.
	if !regexp.MustCompile(`│\s+3\s+│\s+2\s+│`).MatchString(stdout) {
		t.Fatalf("expected agents column count in %q", stdout)
	}

	// Without --session the summary renders no drill-down and, off a
	// terminal, never prompts even when stdin carries input.
	testutil.WithStdin(t, "1\ny\nq\n", func() {
		code, stdout, stderr = testutil.RunApp(t, app, "usage", "--root", root, "--all")
	})
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, banned := range []string{"agents in session", "q to quit", "> "} {
		if strings.Contains(stdout, banned) {
			t.Fatalf("expected no interactive output %q in %q", banned, stdout)
		}
	}
}

func TestUsageCommandZeroAgentSessionAndNoInput(t *testing.T) {
	root := t.TempDir()
	writeDiscoverTranscript(t, filepath.Join(root, "-work-proj", "solo-1.jsonl"), []string{
		usageTranscriptLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj", 10, 0, 5, 1),
	})
	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "usage", "--root", root, "--all", "--session", "solo", "--no-input")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	// One turn, zero agents in the session row; no drill-down section.
	if !regexp.MustCompile(`│\s+1\s+│\s+0\s+│`).MatchString(stdout) {
		t.Fatalf("expected zero agents column in %q", stdout)
	}
	if strings.Contains(stdout, "agents in session") || strings.Contains(stdout, "q to quit") {
		t.Fatalf("expected no drill-down or prompt for agent-free session: %q", stdout)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "usage", "--root", root, "--all", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected usage stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	report := decodeUsageJSON(t, stdout)
	session := report.Sessions[0]
	if session.AgentCount != 0 || len(session.Agents) != 0 {
		t.Fatalf("expected agent-free session json, got %+v", session)
	}
	if !strings.Contains(stdout, `"main"`) || strings.Contains(stdout, `"agents"`) {
		t.Fatalf("expected main object and omitted agents array, got %q", stdout)
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
