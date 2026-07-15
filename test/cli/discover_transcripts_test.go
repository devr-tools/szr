package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/discover"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func discoverFixtureLine(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture line: %v", err)
	}
	return string(data)
}

func discoverBashPair(t *testing.T, id, command, output string) []string {
	t.Helper()
	use := discoverFixtureLine(t, map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_use", "id": id, "name": "Bash", "input": map[string]any{"command": command}},
			},
		},
	})
	result := discoverFixtureLine(t, map[string]any{
		"type": "user",
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "tool_result", "tool_use_id": id, "content": output},
			},
		},
	})
	return []string{use, result}
}

func writeDiscoverTranscript(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir transcript dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
}

func discoverApp(t *testing.T, records []history.Record) *cli.App {
	t.Helper()
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, record := range records {
		if err := store.Append(record); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}
	return cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
}

func gitStatusHistory() []history.Record {
	records := make([]history.Record, 0, 3)
	for i := 0; i < 3; i++ {
		records = append(records, history.Record{
			Timestamp:      time.Date(2026, 7, 1, 10+i, 0, 0, 0, time.UTC),
			Command:        "git status",
			Profile:        "git-status",
			RawTokens:      1000,
			FilteredTokens: 100,
			SavedTokens:    900,
			SavingsPct:     90,
		})
	}
	return records
}

func TestDiscoverTranscriptsCommand(t *testing.T) {
	root := t.TempDir()
	gitOutput := strings.Repeat("modified: internal/cli/app.go with pending changes\n", 20)
	pipeOutput := strings.Repeat("aggregated dependency checksum entry line\n", 10)
	lines := discoverBashPair(t, "toolu_1", "git status", gitOutput)
	lines = append(lines, discoverBashPair(t, "toolu_2", "git status && cat go.sum | sort", pipeOutput)...)
	lines = append(lines, discoverBashPair(t, "toolu_3", "szr git status", gitOutput)...)
	lines = append(lines, discoverBashPair(t, "toolu_4", "printf ok", "ok")...)
	writeDiscoverTranscript(t, filepath.Join(root, "-agent-demo", "session.jsonl"), lines)

	app := discoverApp(t, gitStatusHistory())
	code, stdout, stderr := testutil.RunApp(t, app, "discover", "--root", root, "--all")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected discover stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"Discover Summary",
		"top unwrapped commands:",
		"git-status",
		"passthrough",
		"top action: run \"git status\" through szr (profile git-status)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected discover output %q in %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "discover", "--root", root, "--all", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected discover json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var report discover.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode discover json: %v", err)
	}
	if report.BashCommands != 4 || report.Unwrapped != 2 || report.SkippedWrapped != 1 || report.SkippedTrivial != 1 {
		t.Fatalf("unexpected report counters: %+v", report)
	}
	if len(report.Top) != 2 {
		t.Fatalf("expected two candidates, got %+v", report.Top)
	}

	gitRaw := history.EstimateTokens(gitOutput)
	gitMissed := int(float64(gitRaw)*0.9 + 0.5)
	top := report.Top[0]
	if top.Command != "git status" || top.Profile != "git-status" || !top.Matched {
		t.Fatalf("expected engine-matched git status first, got %+v", report.Top)
	}
	if top.RawTokens != gitRaw || top.MissedTokens != gitMissed || top.Ratio != 0.9 {
		t.Fatalf("expected history-driven ratio 0.9, got %+v (want raw=%d missed=%d)", top, gitRaw, gitMissed)
	}

	second := report.Top[1]
	pipeRaw := history.EstimateTokens(pipeOutput)
	pipeMissed := int(float64(pipeRaw)*discover.DefaultRatio + 0.5)
	if second.Profile != "passthrough" || second.Matched || second.Ratio != discover.DefaultRatio {
		t.Fatalf("expected fallback profile with default ratio, got %+v", second)
	}
	if second.MissedTokens != pipeMissed {
		t.Fatalf("unexpected fallback estimate: %+v want %d", second, pipeMissed)
	}
	if report.MissedTokens != gitMissed+pipeMissed {
		t.Fatalf("unexpected total estimate: %+v", report)
	}
}

func TestDiscoverScopesToCurrentProject(t *testing.T) {
	root := t.TempDir()
	workDir := t.TempDir()
	restore := testutil.Chdir(t, workDir)
	defer restore()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	output := strings.Repeat("scoped project transcript output line\n", 10)
	writeDiscoverTranscript(t,
		filepath.Join(root, discover.EncodeProjectDir(cwd), "session.jsonl"),
		discoverBashPair(t, "toolu_1", "git status", output),
	)
	writeDiscoverTranscript(t,
		filepath.Join(root, "-other-proj", "session.jsonl"),
		discoverBashPair(t, "toolu_2", "mystery-tool --verbose", output),
	)

	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "discover", "--root", root, "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected scoped discover stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var report discover.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode scoped discover json: %v", err)
	}
	if report.Projects != 1 || len(report.Top) != 1 || report.Top[0].Command != "git status" {
		t.Fatalf("expected current-project scope, got %+v", report)
	}
}

func TestDiscoverMissingTranscriptsRoot(t *testing.T) {
	app := discoverApp(t, nil)
	code, stdout, stderr := testutil.RunApp(t, app, "discover", "--root", filepath.Join(t.TempDir(), "none"))
	if code != 0 || stderr != "" || !strings.Contains(stdout, "no agent transcripts found") {
		t.Fatalf("unexpected missing-root output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}
