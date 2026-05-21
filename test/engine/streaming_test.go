package engine_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"szr/internal/config"
	"szr/internal/engine"
	"szr/internal/history"
	"szr/test/testutil"
)

func TestExecuteUsesStreamReducerAndStreamsTeeOnFailure(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "streamfail", "#!/bin/sh\nprintf 'stdout-one\\n'\nsleep 0.05\nprintf 'stderr-one\\n' >&2\nsleep 0.05\nprintf 'stderr-two\\n' >&2\nexit 7\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	profile := engine.Profile{
		Name:             "streaming-failure",
		StreamPreference: engine.StreamStderrFirst,
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &diagnosticReducer{}
		},
	}

	e := engine.New(cfg, paths, store, []engine.Profile{profile})
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"streamfail"},
		Display: []string{"streamfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute streaming failure: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit 7, got %#v", result)
	}
	if !strings.Contains(result.Display, "stderr-one") || !strings.Contains(result.Display, "[full output:") {
		t.Fatalf("unexpected rendered display: %q", result.Display)
	}
	if result.TeePath == "" {
		t.Fatalf("expected tee path in result: %#v", result)
	}
	teeData := string(testutil.MustReadFile(t, result.TeePath))
	for _, want := range []string{"stdout-one", "stderr-one", "stderr-two"} {
		if !strings.Contains(teeData, want) {
			t.Fatalf("expected tee output to contain %q, got %q", want, teeData)
		}
	}
	if result.BytesParsed == 0 || result.RawBytesRead == 0 {
		t.Fatalf("expected stream byte accounting, got %#v", result)
	}
}

func TestExecuteBypassesTinyStreamOutput(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "tinyout", "#!/bin/sh\nprintf 'tiny\\n'\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name: "streaming-summary",
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: "compacted"}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"tinyout"},
		Display: []string{"tinyout"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute tiny output: %v", err)
	}
	if result.Display != "tiny" {
		t.Fatalf("expected tiny-output bypass, got %#v", result)
	}
	if result.BypassReason == "" {
		t.Fatalf("expected bypass reason, got %#v", result)
	}
}

func TestExecuteRemovesIncrementalTeeOnSuccess(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "streamok", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'warn\\n' >&2\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), nil)

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"streamok"},
		Display: []string{"streamok"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute success: %v", err)
	}
	if result.TeePath != "" {
		t.Fatalf("did not expect tee path on success: %#v", result)
	}
	entries, readErr := os.ReadDir(paths.TeeDir)
	if readErr != nil {
		t.Fatalf("read tee dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected tee dir cleanup, found %d entries", len(entries))
	}
}

func TestExecuteCountsTokensWithoutFullyCapturingIgnoredStream(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "mixedout", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'very noisy ignored stderr line\\n' >&2\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{{
		Name:             "stdout-only-stream",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "mixedout"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: "ok"}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"mixedout"},
		Display: []string{"mixedout"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute mixed output: %v", err)
	}
	if strings.Contains(result.RawCombined, "ignored stderr") {
		t.Fatalf("did not expect ignored stderr to be fully buffered, got %#v", result)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 {
		t.Fatalf("expected one history record, got %#v", records)
	}
	if records[0].RawTokens <= history.EstimateTokens("ok") {
		t.Fatalf("expected raw token count to include ignored stderr, got %#v", records[0])
	}
}

func TestVerboseCaptureKeepsRawCombined(t *testing.T) {
	binDir := t.TempDir()
	testutil.WriteExecutable(t, binDir, "mixedverbose", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'stderr detail\\n' >&2\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:             "stdout-only-stream",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "mixedverbose"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: "ok"}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{"mixedverbose"},
		Display: []string{"mixedverbose"},
		Cwd:     root,
		Verbose: 3,
	}, false)
	if err != nil {
		t.Fatalf("execute verbose mixed output: %v", err)
	}
	if !strings.Contains(result.RawCombined, "stderr detail") {
		t.Fatalf("expected verbose execution to keep full raw output, got %#v", result)
	}
}

type diagnosticReducer struct {
	stderr strings.Builder
	parsed int
}

func (r *diagnosticReducer) ConsumeStdout(chunk []byte) {
	r.parsed += len(chunk)
}

func (r *diagnosticReducer) ConsumeStderr(chunk []byte) {
	r.parsed += len(chunk)
	_, _ = r.stderr.Write(chunk)
}

func (r *diagnosticReducer) Result() string {
	return strings.TrimSpace(r.stderr.String())
}

func (r *diagnosticReducer) BytesParsed() int {
	return r.parsed
}

func (r *diagnosticReducer) FallbackUsed() bool {
	return false
}

type staticReducer struct {
	rendered string
}

func (r *staticReducer) ConsumeStdout([]byte) {}

func (r *staticReducer) ConsumeStderr([]byte) {}

func (r *staticReducer) Result() string {
	return r.rendered
}

func (r *staticReducer) BytesParsed() int {
	return 0
}

func (r *staticReducer) FallbackUsed() bool {
	return false
}
