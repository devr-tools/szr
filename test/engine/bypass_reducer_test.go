package engine_test

import (
	"context"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecutePrefersReducerSummaryOverTinyRawBypass(t *testing.T) {
	t.Parallel()

	reducer := &countingSummaryReducer{summary: "4 matches"}
	result := runTinyBypassCommand(t, reducer)
	if result.Display != "4 matches" {
		t.Fatalf("expected cheaper reducer summary over tiny raw bypass, got %#v", result)
	}
}

func TestExecuteKeepsTinyRawBypassWhenSummaryIsNotCheaper(t *testing.T) {
	t.Parallel()

	reducer := &countingSummaryReducer{summary: "a much longer expanded restatement of the tiny original output lines"}
	result := runTinyBypassCommand(t, reducer)
	if result.Display != tinyBypassRawDisplay {
		t.Fatalf("expected raw tiny-output bypass for costlier summary, got %#v", result)
	}
}

func TestExecuteKeepsTinyRawBypassWhenReducerFellBack(t *testing.T) {
	t.Parallel()

	reducer := &countingSummaryReducer{summary: "4 matches", fallback: true}
	result := runTinyBypassCommand(t, reducer)
	if result.Display != tinyBypassRawDisplay {
		t.Fatalf("expected raw tiny-output bypass after reducer fallback, got %#v", result)
	}
}

const tinyBypassRawDisplay = "match one\nmatch two\nmatch three\nmatch four"

func runTinyBypassCommand(t *testing.T, reducer engine.StreamReducer) engine.Result {
	t.Helper()

	binDir := t.TempDir()
	tinyMatchPath := testutil.WriteExecutable(t, binDir, "tinymatches", "#!/bin/sh\nprintf 'match one\\nmatch two\\nmatch three\\nmatch four\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name: "streaming-summary",
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "tinymatches"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return reducer
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{tinyMatchPath},
		Display: []string{"tinymatches"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute tiny matches: %v", err)
	}
	if result.BypassReason == "" {
		t.Fatalf("expected tiny-output fast path to fire, got %#v", result)
	}
	return result
}

// countingSummaryReducer parses every byte it is fed and reports a fixed
// summary, mirroring a stream reducer that fully understood the output.
type countingSummaryReducer struct {
	summary  string
	parsed   int
	fallback bool
}

func (r *countingSummaryReducer) ConsumeStdout(chunk []byte) {
	r.parsed += len(chunk)
}

func (r *countingSummaryReducer) ConsumeStderr(chunk []byte) {
	r.parsed += len(chunk)
}

func (r *countingSummaryReducer) Result() string {
	return r.summary
}

func (r *countingSummaryReducer) BytesParsed() int {
	return r.parsed
}

func (r *countingSummaryReducer) FallbackUsed() bool {
	return r.fallback
}
