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
	wantFirst := time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC)
	wantLast := time.Date(2026, 7, 10, 10, 30, 0, 0, time.UTC)
	if !one.FirstSeen.Equal(wantFirst) || !one.LastSeen.Equal(wantLast) {
		t.Fatalf("unexpected session window: %+v", one)
	}
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
