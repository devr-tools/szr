package discover_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/discover"
)

func usageAssistantLine(t *testing.T, msgID, ts, cwd string, input, cacheCreate, cacheRead, output int) string {
	t.Helper()
	return jsonLine(t, map[string]any{
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

func TestScanUsageAggregatesSessionsWithSubagents(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-proj-a", "sess-one.jsonl"),
		usageAssistantLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work/proj-a", 10, 100, 200, 5),
		// Streamed repeat of the same message id with cumulative counters:
		// only the final payload may be counted.
		usageAssistantLine(t, "msg_1", "2026-07-10T10:00:01Z", "/work/proj-a", 10, 100, 200, 50),
		"not json at all {",
		jsonLine(t, map[string]any{"type": "assistant", "message": map[string]any{"id": "msg_no_usage"}}),
		jsonLine(t, map[string]any{"type": "user", "timestamp": "2026-07-10T10:30:00Z", "cwd": "/work/proj-a"}),
	)
	writeTranscript(t, filepath.Join(root, "-proj-a", "sess-one", "subagents", "agent-x.jsonl"),
		usageAssistantLine(t, "msg_2", "2026-07-10T10:10:00Z", "/work/proj-a", 1, 50, 0, 7),
	)
	writeTranscript(t, filepath.Join(root, "-proj-a", "sess-two.jsonl"),
		usageAssistantLine(t, "msg_3", "2026-07-10T11:00:00Z", "/work/proj-a", 2, 10, 20, 3),
	)

	sessions := discover.ScanUsage(discover.UsageOptions{Root: root})
	if len(sessions) != 2 {
		t.Fatalf("expected two sessions, got %+v", sessions)
	}
	if sessions[0].SessionID != "sess-two" || sessions[1].SessionID != "sess-one" {
		t.Fatalf("expected newest-first ordering, got %+v", sessions)
	}

	one := sessions[1]
	if one.Turns != 2 || one.TranscriptFiles != 2 || one.Project != "-proj-a" || one.Cwd != "/work/proj-a" {
		t.Fatalf("unexpected session shape: %+v", one)
	}
	if one.InputTokens != 11 || one.CacheCreationTokens != 150 || one.CacheReadTokens != 200 || one.OutputTokens != 57 {
		t.Fatalf("unexpected token sums: %+v", one)
	}
	if one.FreshInputTokens() != 161 {
		t.Fatalf("unexpected fresh input: %d", one.FreshInputTokens())
	}
	if len(one.Agents) != 1 || one.Agents[0].AgentID != "x" || one.Main.Turns != 1 {
		t.Fatalf("unexpected agent breakdown: %+v", one)
	}
	two := sessions[0]
	if len(two.Agents) != 0 || two.Main.Turns != two.Turns || two.Main.OutputTokens != two.OutputTokens {
		t.Fatalf("expected agent-free session to match its main transcript: %+v", two)
	}
	wantFirst := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	wantLast := time.Date(2026, 7, 10, 10, 30, 0, 0, time.UTC)
	if !one.FirstSeen.Equal(wantFirst) || !one.LastSeen.Equal(wantLast) {
		t.Fatalf("unexpected session window: %+v", one)
	}
}

func assertUsageInvariant(t *testing.T, session discover.SessionUsage) {
	t.Helper()
	sum := session.Main
	for _, agent := range session.Agents {
		sum.Turns += agent.Turns
		sum.InputTokens += agent.InputTokens
		sum.CacheCreationTokens += agent.CacheCreationTokens
		sum.CacheReadTokens += agent.CacheReadTokens
		sum.OutputTokens += agent.OutputTokens
	}
	totalsMatch := sum.Turns == session.Turns &&
		sum.InputTokens == session.InputTokens &&
		sum.CacheCreationTokens == session.CacheCreationTokens &&
		sum.CacheReadTokens == session.CacheReadTokens &&
		sum.OutputTokens == session.OutputTokens
	if !totalsMatch {
		t.Fatalf("main + agents %+v does not equal session totals %+v", sum, session)
	}
}

func TestScanUsagePerAgentBreakdown(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "-proj-a", "sess-multi")
	writeTranscript(t, session+".jsonl",
		usageAssistantLine(t, "msg_m1", "2026-07-10T10:00:00Z", "/work/proj-a", 10, 100, 200, 5),
		usageAssistantLine(t, "msg_m1", "2026-07-10T10:00:01Z", "/work/proj-a", 10, 100, 200, 50),
		usageAssistantLine(t, "msg_m2", "2026-07-10T10:05:00Z", "/work/proj-a", 5, 20, 30, 8),
	)
	writeTranscript(t, filepath.Join(session, "subagents", "agent-aaa111.jsonl"),
		usageAssistantLine(t, "msg_a1", "2026-07-10T10:10:00Z", "/work/proj-a", 1, 40, 10, 6),
		// Streamed repeat within one transcript still dedups per message id.
		usageAssistantLine(t, "msg_a1", "2026-07-10T10:10:01Z", "/work/proj-a", 1, 40, 10, 9),
		usageAssistantLine(t, "msg_dup", "2026-07-10T10:11:00Z", "/work/proj-a", 2, 8, 4, 3),
	)
	writeTranscript(t, filepath.Join(session, "subagents", "agent-bbb222.jsonl"),
		// Same message id as agent-aaa111: dedup is scoped per transcript
		// file, so this counts as a separate turn.
		usageAssistantLine(t, "msg_dup", "2026-07-10T10:12:00Z", "/work/proj-a", 7, 30, 90, 11),
	)
	writeTranscript(t, filepath.Join(session, "subagents", "agent-ccc333.jsonl"),
		jsonLine(t, map[string]any{"type": "user", "timestamp": "2026-07-10T10:13:00Z"}),
	)

	sessions := discover.ScanUsage(discover.UsageOptions{Root: root})
	if len(sessions) != 1 {
		t.Fatalf("expected one session, got %+v", sessions)
	}
	multi := sessions[0]
	if multi.Turns != 5 || multi.TranscriptFiles != 4 {
		t.Fatalf("unexpected session totals: %+v", multi)
	}
	if multi.Main.Turns != 2 || multi.Main.InputTokens != 15 || multi.Main.CacheCreationTokens != 120 ||
		multi.Main.CacheReadTokens != 230 || multi.Main.OutputTokens != 58 || multi.Main.AgentID != "" {
		t.Fatalf("unexpected main usage: %+v", multi.Main)
	}
	if len(multi.Agents) != 2 {
		t.Fatalf("expected two billed agents, got %+v", multi.Agents)
	}
	first, second := multi.Agents[0], multi.Agents[1]
	if first.AgentID != "aaa111" || first.Turns != 2 || first.InputTokens != 3 ||
		first.CacheCreationTokens != 48 || first.CacheReadTokens != 14 || first.OutputTokens != 12 {
		t.Fatalf("unexpected first agent: %+v", first)
	}
	if first.FreshInputTokens() != 51 {
		t.Fatalf("unexpected agent fresh input: %d", first.FreshInputTokens())
	}
	if second.AgentID != "bbb222" || second.Turns != 1 || second.InputTokens != 7 ||
		second.CacheCreationTokens != 30 || second.CacheReadTokens != 90 || second.OutputTokens != 11 {
		t.Fatalf("unexpected second agent: %+v", second)
	}
	assertUsageInvariant(t, multi)
}

func TestScanUsageSessionPrefixFilter(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-proj", "abc-123.jsonl"),
		usageAssistantLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work", 1, 0, 0, 1),
	)
	writeTranscript(t, filepath.Join(root, "-proj", "xyz-456.jsonl"),
		usageAssistantLine(t, "msg_2", "2026-07-10T10:00:00Z", "/work", 1, 0, 0, 1),
	)

	sessions := discover.ScanUsage(discover.UsageOptions{Root: root, SessionPrefix: "abc"})
	if len(sessions) != 1 || sessions[0].SessionID != "abc-123" {
		t.Fatalf("expected prefix-filtered session, got %+v", sessions)
	}
}

func TestScanUsageSkipsSessionsWithoutUsage(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-proj", "empty.jsonl"),
		jsonLine(t, map[string]any{"type": "user", "timestamp": "2026-07-10T10:00:00Z"}),
	)
	if sessions := discover.ScanUsage(discover.UsageOptions{Root: root}); len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %+v", sessions)
	}
	missing := discover.ScanUsage(discover.UsageOptions{Root: filepath.Join(root, "none")})
	if len(missing) != 0 {
		t.Fatalf("expected empty scan for missing root, got %+v", missing)
	}
}

func TestScanUsageHonorsSinceCutoff(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	fresh := filepath.Join(root, "-proj", "fresh.jsonl")
	stale := filepath.Join(root, "-proj", "stale.jsonl")
	writeTranscript(t, fresh, usageAssistantLine(t, "msg_1", "2026-07-10T10:00:00Z", "/work", 1, 0, 0, 1))
	writeTranscript(t, stale, usageAssistantLine(t, "msg_2", "2026-07-01T10:00:00Z", "/work", 1, 0, 0, 1))
	old := now.Add(-72 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	sessions := discover.ScanUsage(discover.UsageOptions{Root: root, Now: now, Since: 24 * time.Hour})
	if len(sessions) != 1 || sessions[0].SessionID != "fresh" {
		t.Fatalf("expected only fresh session, got %+v", sessions)
	}
}
