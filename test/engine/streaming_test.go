package engine_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func TestExecuteUsesStreamReducerAndStreamsTeeOnFailure(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	streamFailPath := testutil.WriteExecutable(t, binDir, "streamfail", "#!/bin/sh\nprintf 'stdout-one\\n'\nprintf 'stderr-one\\n' >&2\nprintf 'stderr-two\\n' >&2\nexit 7\n")

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
		Command: []string{streamFailPath},
		Display: []string{"streamfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute streaming failure: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit 7, got %#v", result)
	}
	if !strings.Contains(result.Display, "stderr-one") || !strings.Contains(result.Display, "[tee: ") {
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

func TestExecuteWritesFailureTeeWithoutStreamReducer(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	failPath := testutil.WriteExecutable(t, binDir, "renderfail", "#!/bin/sh\nprintf 'stdout-one\\n'\nprintf 'stderr-one\\n' >&2\nexit 9\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "render-only",
		Confidence: engine.ConfidenceMedium,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "renderfail"
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return exec.Stderr
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{failPath},
		Display: []string{"renderfail"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute render-only failure: %v", err)
	}
	if result.ExitCode != 9 {
		t.Fatalf("expected exit 9, got %#v", result)
	}
	if result.TeePath == "" {
		t.Fatalf("expected tee path for non-stream reducer failure, got %#v", result)
	}
	teeData := string(testutil.MustReadFile(t, result.TeePath))
	if !strings.Contains(teeData, "stdout-one") || !strings.Contains(teeData, "stderr-one") {
		t.Fatalf("expected tee output to contain both streams, got %q", teeData)
	}
}

func TestExecuteSkipsFailureTeeForHighConfidenceStructuredFailure(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	failPath := testutil.WriteExecutable(t, binDir, "renderfailhc", "#!/bin/sh\nprintf 'stdout-one\\n'\nprintf 'stderr-one\\n' >&2\nexit 11\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "git-status",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "renderfailhc"
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return exec.Stderr
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{failPath},
		Display: []string{"renderfailhc"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute high-confidence failure: %v", err)
	}
	if result.ExitCode != 11 {
		t.Fatalf("expected exit 11, got %#v", result)
	}
	if result.TeePath != "" || strings.Contains(result.Display, "[tee: ") {
		t.Fatalf("did not expect tee artifact for high-confidence failure, got %#v", result)
	}
	if strings.TrimSpace(result.Display) != "stderr-one" {
		t.Fatalf("expected rendered stderr without tee suffix, got %q", result.Display)
	}
}

func TestExecuteBypassesTinyStreamOutput(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	tinyOutPath := testutil.WriteExecutable(t, binDir, "tinyout", "#!/bin/sh\nprintf 'tiny\\n'\n")

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
		Command: []string{tinyOutPath},
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

func TestExecuteBypassesTinySafeHighConfidenceOutput(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	tinyStatusPath := testutil.WriteExecutable(t, binDir, "tinystatus", "#!/bin/sh\nprintf '## main...origin/main\\nM  README.md\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "git-status",
		Confidence: engine.ConfidenceHigh,
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &staticReducer{rendered: "staged=1"}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{tinyStatusPath},
		Display: []string{"tinystatus"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute tiny safe high-confidence output: %v", err)
	}
	if result.Display != "## main...origin/main\nM  README.md" {
		t.Fatalf("expected raw bypass for safe high-confidence profile, got %#v", result)
	}
	if result.BypassReason == "" {
		t.Fatalf("expected bypass reason, got %#v", result)
	}
}

func TestExecuteRemovesIncrementalTeeOnSuccess(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	streamOKPath := testutil.WriteExecutable(t, binDir, "streamok", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'warn\\n' >&2\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), nil)

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{streamOKPath},
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

func TestExecutePersistsRecoveryArtifactOnSuccessfulOmission(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	findLikePath := testutil.WriteExecutable(t, binDir, "findlike", "#!/bin/sh\nprintf 'one\\ntwo\\nthree\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "recovery-stream",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "findlike"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &recoverySummaryReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{findLikePath},
		Display: []string{"findlike"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute recovery summary: %v", err)
	}
	if result.TeePath == "" {
		t.Fatalf("expected recovery artifact tee path, got %#v", result)
	}
	if !strings.Contains(result.Display, "[recovery: omitted 2 additional matches; tee: ") {
		t.Fatalf("expected recovery hint in display, got %q", result.Display)
	}
	teeData := string(testutil.MustReadFile(t, result.TeePath))
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(teeData, want) {
			t.Fatalf("expected recovery artifact to contain %q, got %q", want, teeData)
		}
	}
}

func TestExecutePersistsRecoveryArtifactOnCompressionContract(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	longOutPath := testutil.WriteExecutable(t, binDir, "longout", "#!/bin/sh\nfor i in $(seq 1 80); do printf 'token-%02d\\n' \"$i\"; done\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:   "contract-render",
		Budget: engine.OutputBudget{MaxLines: 12, MaxTokens: 16},
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "longout"
		},
		Render: func(_ engine.Invocation, exec engine.Execution) string {
			return exec.Stdout
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{longOutPath},
		Display: []string{"longout"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute long output: %v", err)
	}
	if result.TeePath == "" {
		t.Fatalf("expected tee path for compressed successful output, got %#v", result)
	}
	if !strings.Contains(result.Display, "[recovery: ") && !strings.Contains(result.Display, "[tee: ") && !strings.Contains(result.Display, "[full output saved]") {
		t.Fatalf("expected budget-aware recovery suffix in display, got %q", result.Display)
	}
	allowed := 16
	if got := history.EstimateTokens(result.Display); got > allowed {
		t.Fatalf("expected final display <= %d tokens after hint decoration, got %d (%q)", allowed, got, result.Display)
	}
}

func TestExecuteCountsTokensWithoutFullyCapturingIgnoredStream(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	mixedOutPath := testutil.WriteExecutable(t, binDir, "mixedout", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'very noisy ignored stderr line\\n' >&2\n")

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
		Command: []string{mixedOutPath},
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

func TestExecuteCountsTokensWithoutFullyCapturingIgnoredStdout(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	mixedErrPath := testutil.WriteExecutable(t, binDir, "mixederr", "#!/bin/sh\nprintf 'very noisy ignored stdout line\\n'\nprintf 'primary stderr\\n' >&2\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	store := history.New(paths.HistoryFile)
	e := engine.New(cfg, paths, store, []engine.Profile{{
		Name:             "stderr-only-stream",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStderrOnly,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "mixederr"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &diagnosticReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{mixedErrPath},
		Display: []string{"mixederr"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute mixed stderr output: %v", err)
	}
	if result.Display != "primary stderr" {
		t.Fatalf("expected stderr-only render, got %#v", result)
	}
	if strings.Contains(result.RawCombined, "ignored stdout") {
		t.Fatalf("did not expect ignored stdout to be fully buffered, got %#v", result)
	}

	records, loadErr := store.LoadAll()
	if loadErr != nil {
		t.Fatalf("load history: %v", loadErr)
	}
	if len(records) != 1 {
		t.Fatalf("expected one history record, got %#v", records)
	}
	if records[0].RawTokens <= history.EstimateTokens("primary stderr") {
		t.Fatalf("expected raw token count to include ignored stdout, got %#v", records[0])
	}
}

func TestVerboseCaptureKeepsRawCombined(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	mixedVerbosePath := testutil.WriteExecutable(t, binDir, "mixedverbose", "#!/bin/sh\nprintf 'ok\\n'\nprintf 'stderr detail\\n' >&2\n")

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
		Command: []string{mixedVerbosePath},
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

func TestExecuteStreamsBothChannelsByDefault(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	bothPath := testutil.WriteExecutable(t, binDir, "bothstreams", "#!/bin/sh\nprintf 'stdout-one\\nstdout-two\\nstdout-three\\nstdout-four\\n'\nprintf 'stderr-one\\nstderr-two\\nstderr-three\\nstderr-four\\n' >&2\nexit 7\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "dual-stream",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "bothstreams"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &combinedReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{bothPath},
		Display: []string{"bothstreams"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute dual stream: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected failure exit code, got %#v", result)
	}
	if !strings.Contains(result.Display, "stdout-one") || !strings.Contains(result.Display, "stderr-one") {
		t.Fatalf("expected default stream mode to reduce both channels, got %#v", result)
	}
}

func TestExecuteStopsFeedingReducerAfterDone(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	manyLinesPath := testutil.WriteExecutable(t, binDir, "manylines", "#!/bin/sh\nprintf 'FAIL one\\n'\nprintf 'FAIL two\\n'\nprintf 'FAIL three\\n'\nprintf 'FAIL four\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	reducer := &doneReducer{}
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "done-stream",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "manylines"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return reducer
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{manyLinesPath},
		Display: []string{"manylines"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute done reducer: %v", err)
	}
	if !strings.Contains(result.Display, "FAIL one") {
		t.Fatalf("unexpected display: %#v", result)
	}
	if reducer.consumeCalls != 1 {
		t.Fatalf("expected reducer to stop after first chunk, got %d calls", reducer.consumeCalls)
	}
	if result.RawBytesRead <= result.BytesParsed {
		t.Fatalf("expected raw bytes to exceed parsed bytes after early stop, got %#v", result)
	}
}

func TestExecuteKeepsTinyRawBypassAfterReducerDone(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	tinyStatusPath := testutil.WriteExecutable(t, binDir, "tinydone", "#!/bin/sh\nprintf '## main\\nM  README.md\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	reducer := &doneReducer{}
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "git-status",
		Confidence: engine.ConfidenceHigh,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "tinydone"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return reducer
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{tinyStatusPath},
		Display: []string{"tinydone"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute tiny done reducer: %v", err)
	}
	if result.Display != "## main\nM  README.md" {
		t.Fatalf("expected tiny raw bypass, got %#v", result)
	}
	if reducer.consumeCalls != 1 {
		t.Fatalf("expected reducer to stop after first chunk, got %d calls", reducer.consumeCalls)
	}
}

func TestExecuteStreamingPublishesPartialPreviewBeforeExit(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	delayedFailPath := testutil.WriteExecutable(t, binDir, "delayedfail", "#!/bin/sh\nprintf 'FAIL first\\n'\nprintf 'FAIL second\\n'\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:             "preview-stream",
		Confidence:       engine.ConfidenceHigh,
		StreamPreference: engine.StreamStdoutFirst,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "delayedfail"
		},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &previewReducer{}
		},
	}})

	partials := make(chan engine.PartialResult, 4)
	_, err := e.ExecuteStreaming(context.Background(), engine.Invocation{
		Command: []string{delayedFailPath},
		Display: []string{"delayedfail"},
		Cwd:     root,
	}, false, func(partial engine.PartialResult) {
		partials <- partial
	})
	close(partials)
	if err != nil {
		t.Fatalf("execute streaming preview: %v", err)
	}

	seenPartial := false
	seenFinal := false
	for partial := range partials {
		if !partial.Final {
			seenPartial = true
			if !strings.Contains(partial.Display, "FAIL first") {
				t.Fatalf("expected early partial to include first failure, got %#v", partial)
			}
		}
		if partial.Final {
			seenFinal = true
		}
	}
	if !seenPartial || !seenFinal {
		t.Fatalf("expected both partial and final updates, got partial=%t final=%t", seenPartial, seenFinal)
	}
}

func TestExecuteUsesFailureEscapeForLowConfidenceFallback(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	lowConfPath := testutil.WriteExecutable(t, binDir, "lowconf", "#!/bin/sh\nprintf 'line1\\nline2\\nline3\\nline4\\nline5\\nline6\\n' >&2\nexit 9\n")

	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)

	cfg := config.Default()
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), []engine.Profile{{
		Name:       "low-confidence-fallback",
		Confidence: engine.ConfidenceLow,
		Match: func(inv engine.Invocation) bool {
			return len(inv.Display) > 0 && inv.Display[0] == "lowconf"
		},
		Budget: engine.OutputBudget{MaxLines: 2, MaxBytes: 320, MaxTokens: 64},
		StreamRender: func(engine.Invocation, engine.OutputBudget) engine.StreamReducer {
			return &fallbackReducer{}
		},
	}})

	result, err := e.Execute(context.Background(), engine.Invocation{
		Command: []string{lowConfPath},
		Display: []string{"lowconf"},
		Cwd:     root,
	}, false)
	if err != nil {
		t.Fatalf("execute low-confidence fallback: %v", err)
	}
	if !strings.Contains(result.Display, "line1") || !strings.Contains(result.Display, "... +1 more lines") {
		t.Fatalf("expected compacted failure escape output, got %#v", result)
	}
	if strings.Contains(result.Display, "line6") {
		t.Fatalf("did not expect full raw output after failure escape, got %#v", result)
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

type doneReducer struct {
	consumeCalls int
	rendered     []string
}

func (r *doneReducer) ConsumeStdout(chunk []byte) {
	r.consumeCalls++
	r.rendered = append(r.rendered, strings.TrimSpace(string(chunk)))
}

func (r *doneReducer) ConsumeStderr([]byte) {}

func (r *doneReducer) Result() string {
	return strings.Join(r.rendered, "\n")
}

func (r *doneReducer) BytesParsed() int {
	return len(strings.Join(r.rendered, "\n"))
}

func (r *doneReducer) FallbackUsed() bool {
	return false
}

func (r *doneReducer) Done() bool {
	return r.consumeCalls >= 1
}

func (r *doneReducer) Preview() string {
	return r.Result()
}

type previewReducer struct {
	lines []string
}

type combinedReducer struct {
	lines []string
}

func (r *combinedReducer) ConsumeStdout(chunk []byte) {
	r.consume(chunk)
}

func (r *combinedReducer) ConsumeStderr(chunk []byte) {
	r.consume(chunk)
}

func (r *combinedReducer) consume(chunk []byte) {
	for _, line := range strings.Split(strings.TrimSpace(string(chunk)), "\n") {
		if line != "" {
			r.lines = append(r.lines, line)
		}
	}
}

func (r *combinedReducer) Result() string {
	return strings.Join(r.lines, "\n")
}

func (r *combinedReducer) BytesParsed() int {
	return len(r.Result())
}

func (r *combinedReducer) FallbackUsed() bool {
	return false
}

func (r *previewReducer) ConsumeStdout(chunk []byte) {
	for _, line := range strings.Split(strings.TrimSpace(string(chunk)), "\n") {
		if line != "" {
			r.lines = append(r.lines, line)
		}
	}
}

func (r *previewReducer) ConsumeStderr([]byte) {}

func (r *previewReducer) Result() string {
	return strings.Join(r.lines, "\n")
}

func (r *previewReducer) BytesParsed() int {
	return len(r.Result())
}

func (r *previewReducer) FallbackUsed() bool {
	return false
}

func (r *previewReducer) Preview() string {
	return r.Result()
}

type fallbackReducer struct{}

func (r *fallbackReducer) ConsumeStdout([]byte) {}

func (r *fallbackReducer) ConsumeStderr([]byte) {}

func (r *fallbackReducer) Result() string {
	return ""
}

func (r *fallbackReducer) BytesParsed() int {
	return 0
}

func (r *fallbackReducer) FallbackUsed() bool {
	return true
}

type recoverySummaryReducer struct{}

func (r *recoverySummaryReducer) ConsumeStdout([]byte) {}

func (r *recoverySummaryReducer) ConsumeStderr([]byte) {}

func (r *recoverySummaryReducer) Result() string {
	return "1 matches\none\n... +2 more matches"
}

func (r *recoverySummaryReducer) BytesParsed() int {
	return 0
}

func (r *recoverySummaryReducer) FallbackUsed() bool {
	return false
}

func (r *recoverySummaryReducer) RecoveryInfo() (string, string, bool) {
	return engine.RecoveryKindFullOutput, "omitted 2 additional matches", true
}
