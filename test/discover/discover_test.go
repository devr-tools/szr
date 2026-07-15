package discover_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/discover"
	"github.com/devr-tools/szr/internal/history"
)

func jsonLine(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(data)
}

func toolUseLine(t *testing.T, id, command string) string {
	t.Helper()
	return jsonLine(t, map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": command}},
			},
		},
	})
}

func toolResultLine(t *testing.T, id string, content any) string {
	t.Helper()
	return jsonLine(t, map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_result", "tool_use_id": id, "content": content},
			},
		},
	})
}

func writeTranscript(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
}

func bigOutput(marker string) string {
	return strings.Repeat(marker+" noisy build output line with plenty of detail\n", 12)
}

func scanOptions(root string) discover.Options {
	return discover.Options{
		Root: root,
		Top:  10,
		Matcher: func(command string) (string, bool) {
			if strings.HasPrefix(command, "git status") {
				return "git-status", true
			}
			return "passthrough", false
		},
		Ratio: func(profile string) float64 {
			if profile == "git-status" {
				return 0.9
			}
			return 0.5
		},
	}
}

func estimateMissed(raw int, ratio float64) int {
	return int(float64(raw)*ratio + 0.5)
}

func TestScanExtractsAndAggregates(t *testing.T) {
	root := t.TempDir()
	outA := bigOutput("branch")
	outB := bigOutput("more-branch")
	outC := bigOutput("mystery")
	writeTranscript(t, filepath.Join(root, "-tmp-proj", "session.jsonl"),
		toolUseLine(t, "toolu_1", "git status"),
		toolResultLine(t, "toolu_1", outA),
		toolUseLine(t, "toolu_2", "git status"),
		toolResultLine(t, "toolu_2", outB),
		toolUseLine(t, "toolu_3", "mystery-tool --verbose"),
		toolResultLine(t, "toolu_3", outC),
	)

	report := discover.Scan(scanOptions(root))
	if report.Projects != 1 || report.Files != 1 || report.BashCommands != 3 || report.Unwrapped != 3 {
		t.Fatalf("unexpected report counters: %+v", report)
	}
	if len(report.Top) != 2 {
		t.Fatalf("expected two aggregated commands, got %+v", report.Top)
	}

	gitRaw := history.EstimateTokens(outA) + history.EstimateTokens(outB)
	gitMissed := estimateMissed(gitRaw, 0.9)
	top := report.Top[0]
	if top.Command != "git status" || top.Count != 2 || !top.Matched || top.Profile != "git-status" {
		t.Fatalf("unexpected top command: %+v", top)
	}
	if top.RawTokens != gitRaw || top.MissedTokens != gitMissed || top.Ratio != 0.9 {
		t.Fatalf("unexpected top tokens: %+v want raw=%d missed=%d", top, gitRaw, gitMissed)
	}

	mysteryRaw := history.EstimateTokens(outC)
	mysteryMissed := estimateMissed(mysteryRaw, 0.5)
	second := report.Top[1]
	if second.Command != "mystery-tool --verbose" || second.Matched || second.Profile != "passthrough" || second.MissedTokens != mysteryMissed {
		t.Fatalf("unexpected second command: %+v", second)
	}
	if report.RawTokens != gitRaw+mysteryRaw || report.MissedTokens != gitMissed+mysteryMissed {
		t.Fatalf("unexpected totals: %+v", report)
	}
}

func TestScanSkipsWrappedAndTrivial(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-tmp-proj", "session.jsonl"),
		toolUseLine(t, "toolu_1", "szr git status"),
		toolResultLine(t, "toolu_1", bigOutput("wrapped")),
		toolUseLine(t, "toolu_2", "cd /tmp && szr go test ./..."),
		toolResultLine(t, "toolu_2", bigOutput("also-wrapped")),
		toolUseLine(t, "toolu_3", "printf ok"),
		toolResultLine(t, "toolu_3", "ok"),
	)

	report := discover.Scan(scanOptions(root))
	if report.BashCommands != 3 || report.SkippedWrapped != 2 || report.SkippedTrivial != 1 {
		t.Fatalf("unexpected skip counters: %+v", report)
	}
	if report.Unwrapped != 0 || report.MissedTokens != 0 || len(report.Top) != 0 {
		t.Fatalf("expected no candidates, got %+v", report)
	}
}

func TestScanToleratesSchemaDrift(t *testing.T) {
	root := t.TempDir()
	partOne := bigOutput("go.sum-first-half")
	partTwo := bigOutput("go.sum-second-half")
	listResult := jsonLine(t, map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":        "tool_result",
					"tool_use_id": "toolu_1",
					"content": []any{
						map[string]any{"type": "text", "text": partOne},
						map[string]any{"type": "text", "text": partTwo},
					},
				},
			},
		},
	})
	writeTranscript(t, filepath.Join(root, "-tmp-proj", "session.jsonl"),
		"this line is not json at all {",
		jsonLine(t, map[string]any{"type": "queue-operation", "operation": "enqueue"}),
		jsonLine(t, map[string]any{"type": "user", "message": map[string]any{"content": "plain string message"}}),
		toolUseLine(t, "toolu_1", "cat go.sum"),
		listResult,
		toolResultLine(t, "toolu_9999", bigOutput("orphan-result")),
		toolUseLine(t, "toolu_2", "git status"),
	)

	report := discover.Scan(scanOptions(root))
	if report.BashCommands != 2 || report.Unwrapped != 1 || len(report.Top) != 1 {
		t.Fatalf("unexpected drift-tolerant report: %+v", report)
	}
	top := report.Top[0]
	wantRaw := history.EstimateTokens(partOne + "\n" + partTwo)
	if top.Command != "cat go.sum" || top.RawTokens != wantRaw {
		t.Fatalf("unexpected list-content extraction: %+v want raw=%d", top, wantRaw)
	}
}

func TestScanSinceFilterUsesFileModTime(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-tmp-proj", "fresh.jsonl"),
		toolUseLine(t, "toolu_1", "git status"),
		toolResultLine(t, "toolu_1", bigOutput("fresh")),
	)
	stale := filepath.Join(root, "-tmp-proj", "stale.jsonl")
	writeTranscript(t, stale,
		toolUseLine(t, "toolu_2", "mystery-tool --verbose"),
		toolResultLine(t, "toolu_2", bigOutput("stale")),
	)
	old := time.Now().Add(-40 * 24 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatalf("age stale transcript: %v", err)
	}

	report := discover.Scan(scanOptions(root))
	if report.Files != 1 || len(report.Top) != 1 || report.Top[0].Command != "git status" {
		t.Fatalf("expected stale transcript excluded, got %+v", report)
	}
}

func TestScanScopesToProjectAndRecurses(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-proj-one", "nested", "agent.jsonl"),
		toolUseLine(t, "toolu_1", "git status"),
		toolResultLine(t, "toolu_1", bigOutput("one")),
	)
	writeTranscript(t, filepath.Join(root, "-proj-two", "session.jsonl"),
		toolUseLine(t, "toolu_2", "mystery-tool --verbose"),
		toolResultLine(t, "toolu_2", bigOutput("two")),
	)

	opts := scanOptions(root)
	opts.Project = "-proj-one"
	report := discover.Scan(opts)
	if report.Projects != 1 || report.Files != 1 || len(report.Top) != 1 || report.Top[0].Command != "git status" {
		t.Fatalf("unexpected scoped report: %+v", report)
	}

	opts.Project = ""
	report = discover.Scan(opts)
	if report.Projects != 2 || report.Files != 2 || len(report.Top) != 2 {
		t.Fatalf("unexpected all-projects report: %+v", report)
	}
}

func TestScanTopLimitKeepsTotals(t *testing.T) {
	root := t.TempDir()
	writeTranscript(t, filepath.Join(root, "-tmp-proj", "session.jsonl"),
		toolUseLine(t, "toolu_1", "git status"),
		toolResultLine(t, "toolu_1", bigOutput("one")),
		toolUseLine(t, "toolu_2", "mystery-tool --verbose"),
		toolResultLine(t, "toolu_2", bigOutput("two")),
	)

	opts := scanOptions(root)
	opts.Top = 1
	report := discover.Scan(opts)
	if len(report.Top) != 1 || report.Unwrapped != 2 {
		t.Fatalf("expected truncated top with full counts, got %+v", report)
	}
	if report.RawTokens <= report.Top[0].RawTokens {
		t.Fatalf("expected totals to include truncated groups: %+v", report)
	}
}

func TestScanMissingRoot(t *testing.T) {
	report := discover.Scan(discover.Options{Root: filepath.Join(t.TempDir(), "missing")})
	if report.Projects != 0 || report.Files != 0 || len(report.Top) != 0 {
		t.Fatalf("expected empty report for missing root, got %+v", report)
	}
}

func TestEncodeProjectDir(t *testing.T) {
	cases := map[string]string{
		"/Users/pat/Documents/GitHub/szr": "-Users-pat-Documents-GitHub-szr",
		"/tmp/my.app_v2":                  "-tmp-my-app-v2",
		"":                                "",
	}
	for input, want := range cases {
		if got := discover.EncodeProjectDir(input); got != want {
			t.Fatalf("EncodeProjectDir(%q) = %q, want %q", input, got, want)
		}
	}
}
