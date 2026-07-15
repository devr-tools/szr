package engine_test

import (
	"context"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// A stdout-only profile whose command writes its message to stderr (kubectl
// style "No resources found ...") must render a compact view of that stderr
// message instead of flagging an empty-render fallback.
func TestExecuteStdoutOnlyProfileRendersStderrWhenStdoutEmpty(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	stderrOnlyPath := testutil.WriteExecutable(t, binDir, "stderrmsg", "#!/bin/sh\nprintf 'No resources found in default namespace.\\n' >&2\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	e := engine.New(config.Default(), paths, store, []engine.Profile{{
		Name:             "stdout-only-json",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "stderrmsg"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: ""}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{stderrOnlyPath},
		Display: []string{"stderrmsg"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute stderr-message command: %v", err)
	}
	if !strings.Contains(result.Display, "No resources found in default namespace.") {
		t.Fatalf("expected compact stderr message in display, got %q", result.Display)
	}
	if result.FallbackUsed {
		t.Fatalf("did not expect fallback flag for stderr-message render, got %#v", result)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 {
		t.Fatalf("expected one history record, got %#v", records)
	}
	if records[0].FallbackUsed || records[0].EmptyResult || records[0].ExitCode != 0 {
		t.Fatalf("expected clean record for stderr-message render, got %#v", records[0])
	}
	if records[0].SavedTokens < 0 {
		t.Fatalf("expected non-negative savings, got %#v", records[0])
	}
}

// A run with no output on either stream is an "empty result", not a parse
// fallback masquerade: the record must carry the distinct EmptyResult flag.
func TestExecuteEmptyOutputRecordsEmptyResult(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	silentPath := testutil.WriteExecutable(t, binDir, "silent", "#!/bin/sh\nexit 0\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	store := history.New(paths.HistoryFile)
	e := engine.New(config.Default(), paths, store, []engine.Profile{{
		Name:             "silent-stream",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "silent"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: ""}
		},
	}})

	if _, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{silentPath},
		Display: []string{"silent"},
		Cwd:     root,
	}, false); err != nil {
		t.Fatalf("execute silent command: %v", err)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 {
		t.Fatalf("expected one history record, got %#v", records)
	}
	if !records[0].EmptyResult || !records[0].FallbackUsed {
		t.Fatalf("expected empty-result record to keep both flags distinguishable, got %#v", records[0])
	}
}
