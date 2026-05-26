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
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestCommandErrorMatrix(t *testing.T) {
	app := testutil.NewTestApp(t)
	binDir := t.TempDir()
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	testutil.WriteExecutable(t, binDir, "rg", "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"__error__\" ]; then\n    echo \"bad rg\" >&2\n    exit 2\n  fi\n  if [ \"$arg\" = \"nomatch\" ]; then\n    exit 1\n  fi\ndone\necho \"file.go:12:match one\"\n")

	root := t.TempDir()
	file := filepath.Join(root, "one.txt")
	if err := os.WriteFile(file, []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	errorCases := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout []string
		wantStderr []string
	}{
		{"proxy missing", []string{"proxy"}, 2, nil, []string{"missing command for proxy"}},
		{"rewrite missing", []string{"rewrite"}, 2, nil, []string{"rewrite requires a command"}},
		{"rewrite bad format", []string{"rewrite", "--format", "wat", "--command", "git diff"}, 2, nil, []string{"unknown rewrite format"}},
		{"explain missing", []string{"explain"}, 2, nil, []string{"explain requires a command"}},
		{"spread bad flag", []string{"spread", "--bad"}, 2, nil, []string{"unknown spread flag"}},
		{"install mixed targets", []string{"install", "--all", "codex"}, 2, nil, []string{"either --all or explicit targets"}},
		{"install bad flag", []string{"install", "--bad"}, 2, nil, []string{"unknown install flag"}},
		{"uninstall mixed targets", []string{"uninstall", "--all", "codex"}, 2, nil, []string{"either --all or explicit targets"}},
		{"uninstall bad flag", []string{"uninstall", "--bad"}, 2, nil, []string{"unknown self uninstall flag"}},
		{"bench bad flag", []string{"bench", "--bad"}, 2, nil, []string{"unknown bench flag"}},
		{"bench no fixtures", []string{"bench", "missing-fixture"}, 2, nil, []string{"no benchmark fixtures matched"}},
		{"read missing", []string{"read"}, 2, nil, []string{"read requires a file"}},
		{"find missing name", []string{"find", "--name"}, 2, nil, []string{"find requires a value for --name"}},
		{"find missing path", []string{"find", "--path"}, 2, nil, []string{"find requires a value for --path"}},
		{"find missing exclude", []string{"find", "--exclude"}, 2, nil, []string{"find requires a value for --exclude"}},
		{"find bad type", []string{"find", "--type", "x"}, 2, nil, []string{"unsupported find type"}},
		{"find missing type", []string{"find", "--type"}, 2, nil, []string{"find requires a value for --type"}},
		{"find missing max-depth", []string{"find", "--max-depth"}, 2, nil, []string{"find requires a value for --max-depth"}},
		{"find invalid max-depth", []string{"find", "--max-depth", "-1"}, 2, nil, []string{"invalid find max-depth"}},
		{"find extra root", []string{"find", root, "two"}, 2, nil, []string{"unexpected find argument two"}},
		{"read missing level", []string{"read", "-l"}, 2, nil, []string{"missing value for --level"}},
		{"read missing max-lines", []string{"read", "--max-lines"}, 2, nil, []string{"missing value for --max-lines"}},
		{"read file error", []string{"read", filepath.Join(root, "missing.txt")}, 1, nil, []string{"no such file"}},
		{"grep missing", []string{"grep"}, 2, nil, []string{"grep requires a pattern"}},
		{"grep missing rg", []string{"grep", "match", "."}, 1, nil, []string{"executable file not found"}},
		{"grep error", []string{"grep", "pattern", ".", "__error__"}, 2, nil, []string{"bad rg"}},
		{"json missing args", []string{"json"}, 2, nil, []string{"json requires a file"}},
		{"json read error", []string{"json", filepath.Join(root, "missing.json")}, 1, nil, []string{"no such file"}},
		{"log read error", []string{"log", filepath.Join(root, "missing.log")}, 1, nil, []string{"no such file"}},
		{"ls error", []string{"ls", filepath.Join(root, "missing-dir")}, 1, nil, []string{"no such file"}},
		{"run exec error", []string{"run", "missing-binary"}, 1, nil, []string{"executable file not found"}},
		{"grep no match", []string{"grep", "nomatch", "."}, 0, []string{"no matches"}, nil},
		{"rewrite missing format value", []string{"rewrite", "--format"}, 2, nil, []string{"rewrite requires a value for --format"}},
		{"rewrite missing hook value", []string{"rewrite", "--hook"}, 2, nil, []string{"rewrite requires a value for --hook"}},
		{"rewrite missing command value", []string{"rewrite", "--command"}, 2, nil, []string{"rewrite requires a value for --command"}},
		{"rewrite missing binary value", []string{"rewrite", "--binary"}, 2, nil, []string{"rewrite requires a value for --binary"}},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "grep missing rg" {
				t.Setenv("PATH", t.TempDir())
			}
			code, stdout, stderr := testutil.RunApp(t, app, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("unexpected code %d want %d stdout=%q stderr=%q", code, tc.wantCode, stdout, stderr)
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("expected stdout %q in %q", want, stdout)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr %q in %q", want, stderr)
				}
			}
		})
	}
}

func TestSpreadCommandOutputs(t *testing.T) {
	app := newSpreadFixtureApp(t)
	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || !strings.Contains(stdout, "commands run:") || !strings.Contains(stdout, "duration (p50/p95):") || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--history")
	if code != 0 || !strings.Contains(stdout, "recent:") {
		t.Fatalf("unexpected spread history output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil || payload.Commands == 0 {
		t.Fatalf("unexpected spread json payload: %#v err=%v", payload, err)
	}
}

func TestSpreadCommandEmptyAndErrorCases(t *testing.T) {
	app := newSpreadFixtureApp(t)
	emptyApp := testutil.NewTestApp(t)
	code, stdout, stderr := testutil.RunApp(t, emptyApp, "spread")
	if code != 0 || strings.TrimSpace(stdout) != "no tracked commands yet" || stderr != "" {
		t.Fatalf("unexpected empty spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	badRoot := t.TempDir()
	badPaths := config.Paths{
		ConfigDir:   filepath.Join(badRoot, "config"),
		ConfigFile:  filepath.Join(badRoot, "config", "config.json"),
		DataDir:     filepath.Join(badRoot, "data"),
		HistoryFile: badRoot,
		TeeDir:      filepath.Join(badRoot, "tee"),
	}
	if err := config.EnsurePaths(badPaths); err != nil {
		t.Fatalf("ensure bad paths: %v", err)
	}
	badStore := history.New(badRoot)
	badApp := cli.NewWithDependencies("test", config.Default(), badPaths, badStore, testutil.AppEngine(t, badPaths))
	code, stdout, stderr = testutil.RunApp(t, badApp, "spread")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "failed to read history") {
		t.Fatalf("unexpected bad spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "gain")
	if code != 0 || !strings.Contains(stdout, "commands run:") || stderr != "" {
		t.Fatalf("unexpected gain alias output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func newSpreadFixtureApp(t *testing.T) *cli.App {
	t.Helper()
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{
			Timestamp:         time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
			Command:           "szr git status --short",
			Profile:           "git-status",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawBytesRead:      120,
			BytesParsed:       60,
			BytesEmitted:      20,
			RawTokens:         100,
			FilteredTokens:    20,
			SavedTokens:       80,
			SavingsPct:        80,
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
			Command:           "szr go test ./...",
			Profile:           "go-test-json",
			ProfileConfidence: "high",
			DurationMS:        90,
			ExitCode:          1,
			RawBytesRead:      180,
			BytesParsed:       110,
			BytesEmitted:      40,
			RawTokens:         120,
			FilteredTokens:    40,
			SavedTokens:       80,
			SavingsPct:        66.67,
			TeePath:           "/tmp/go-test.log",
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
			Command:           "szr custom command",
			Profile:           "passthrough",
			ProfileConfidence: "low",
			DurationMS:        10,
			ExitCode:          2,
			RawBytesRead:      90,
			BytesParsed:       90,
			BytesEmitted:      60,
			RawTokens:         60,
			FilteredTokens:    60,
			SavedTokens:       0,
			SavingsPct:        0,
			FallbackUsed:      true,
		},
	} {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}
	return cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
}

func TestSpreadReportingHistoryOutput(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	records := []history.Record{
		{
			Timestamp:         time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
			Command:           "szr git status --short",
			Profile:           "git-status",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawBytesRead:      120,
			BytesParsed:       60,
			BytesEmitted:      20,
			RawTokens:         100,
			FilteredTokens:    20,
			SavedTokens:       80,
			SavingsPct:        80,
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
			Command:           "szr go test ./...",
			Profile:           "go-test-json",
			ProfileConfidence: "high",
			DurationMS:        90,
			ExitCode:          1,
			RawBytesRead:      180,
			BytesParsed:       110,
			BytesEmitted:      40,
			RawTokens:         120,
			FilteredTokens:    40,
			SavedTokens:       80,
			SavingsPct:        66.67,
			TeePath:           "/tmp/go-test.log",
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
			Command:           "szr custom command",
			Profile:           "passthrough",
			ProfileConfidence: "low",
			DurationMS:        10,
			ExitCode:          2,
			RawBytesRead:      90,
			BytesParsed:       90,
			BytesEmitted:      60,
			RawTokens:         60,
			FilteredTokens:    60,
			SavedTokens:       0,
			SavingsPct:        0,
			FallbackUsed:      true,
		},
	}
	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--history")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"commands run: 3",
		"total tokens saved: 160 tokens",
		"duration (p50/p95): 30ms / 90ms",
		"bytes (read/parsed/emitted): 390 / 260 / 120",
		"failed commands: 66.7% (2/3)",
		"fallback usage: 33.3% (1/3)",
		"tee usage: 33.3% (1/3)",
		"command",
		"count",
		"avg savings",
		"saved",
		"│ profile",
		"│ conf",
		"p50/p95",
		"profiles:",
		"improvement hotspots:",
		"action",
		"loosen budget or improve fallback path",
		"git-status",
		"go-test-json",
		"passthrough",
		"80.0% [==========--]",
		"66.7% [========----]",
		"0.0% [------------]",
		"recent:",
		"2026-05-20T12:00:00Z  passthrough  confidence=low  10ms  exit=2  fallback=true  0.0%  szr custom command",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected spread output %q in %q", want, stdout)
		}
	}
}

func TestSpreadReportingJSONOutput(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{
			Timestamp:         time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
			Command:           "szr git status --short",
			Profile:           "git-status",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawBytesRead:      120,
			BytesParsed:       60,
			BytesEmitted:      20,
			RawTokens:         100,
			FilteredTokens:    20,
			SavedTokens:       80,
			SavingsPct:        80,
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
			Command:           "szr go test ./...",
			Profile:           "go-test-json",
			ProfileConfidence: "high",
			DurationMS:        90,
			ExitCode:          1,
			RawBytesRead:      180,
			BytesParsed:       110,
			BytesEmitted:      40,
			RawTokens:         120,
			FilteredTokens:    40,
			SavedTokens:       80,
			SavingsPct:        66.67,
			TeePath:           "/tmp/go-test.log",
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
			Command:           "szr custom command",
			Profile:           "passthrough",
			ProfileConfidence: "low",
			DurationMS:        10,
			ExitCode:          2,
			RawBytesRead:      90,
			BytesParsed:       90,
			BytesEmitted:      60,
			RawTokens:         60,
			FilteredTokens:    60,
			SavedTokens:       0,
			SavingsPct:        0,
			FallbackUsed:      true,
		},
	} {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}
	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal spread json: %v", err)
	}
	if payload.DurationP50MS != 30 || payload.DurationP95MS != 90 || payload.Fallbacks != 1 || payload.TeeCount != 1 || payload.RawBytesRead != 390 || payload.BytesParsed != 260 || payload.BytesEmitted != 120 {
		t.Fatalf("unexpected json summary: %#v", payload)
	}
	if len(payload.ProfileStats) != 3 {
		t.Fatalf("unexpected json profile stats: %#v", payload.ProfileStats)
	}
}

func TestSpreadIgnoresUninstallCommands(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range []history.Record{
		{
			Timestamp:         time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
			Command:           "szr git status --short",
			Profile:           "git-status",
			ProfileConfidence: "high",
			DurationMS:        30,
			ExitCode:          0,
			RawBytesRead:      120,
			BytesParsed:       60,
			BytesEmitted:      20,
			RawTokens:         100,
			FilteredTokens:    20,
			SavedTokens:       80,
			SavingsPct:        80,
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
			Command:           "uninstall claude-code",
			Profile:           "passthrough",
			ProfileConfidence: "low",
			DurationMS:        1,
			ExitCode:          0,
			RawTokens:         1,
			FilteredTokens:    1,
			SavedTokens:       0,
			SavingsPct:        0,
			FallbackUsed:      true,
		},
		{
			Timestamp:         time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC),
			Command:           "szr uninstall",
			Profile:           "passthrough",
			ProfileConfidence: "low",
			DurationMS:        1,
			ExitCode:          0,
			RawTokens:         1,
			FilteredTokens:    1,
			SavedTokens:       0,
			SavingsPct:        0,
			FallbackUsed:      true,
		},
	} {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload history.Summary
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal spread json: %v", err)
	}
	if payload.Commands != 1 {
		t.Fatalf("expected uninstall commands to be excluded, got %#v", payload)
	}
	if len(payload.Recent) != 1 || payload.Recent[0].Command != "szr git status --short" {
		t.Fatalf("unexpected recent records after uninstall filtering: %#v", payload.Recent)
	}
}

func TestSpreadBudgetSuggestionsOutput(t *testing.T) {
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for i := 0; i < 4; i++ {
		if err := store.Append(history.Record{
			Timestamp:          time.Date(2026, 5, 20, 10+i, 0, 0, 0, time.UTC),
			Command:            "szr go build ./...",
			CommandFingerprint: history.Fingerprint("szr go build ./..."),
			Profile:            "go-build",
			ProfileConfidence:  "medium",
			DurationMS:         40,
			ExitCode:           1,
			RawBytesRead:       240,
			BytesParsed:        160,
			BytesEmitted:       48,
			RawTokens:          200,
			FilteredTokens:     12,
			SavedTokens:        188,
			SavingsPct:         94,
			FallbackUsed:       i < 3,
		}); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}

	app := cli.NewWithDependencies("test", config.Default(), paths, store, testutil.AppEngine(t, paths))
	code, stdout, stderr := testutil.RunApp(t, app, "spread")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"budget suggestions:",
		"szr go build ./...  profile=go-build samples=4 loosen/fallback_heavy",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected spread budget suggestion %q in %q", want, stdout)
		}
	}
}
