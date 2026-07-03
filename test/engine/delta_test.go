package engine_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

// deltaTestProfiles renders the raw stdout verbatim so the delta digest is
// measured against exactly what the payload file contains.
func deltaTestProfiles() []engine.Profile {
	return []engine.Profile{
		{
			Name: "deltaraw",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "deltaraw"
			},
			Render: func(_ engine.Invocation, exec engine.Execution) string {
				return exec.Stdout
			},
		},
	}
}

// deltaPayloadScript prints the cwd-local payload file and exits with the
// cwd-local exit code file when present, so tests can edit the output and the
// exit status between runs like an edit-test loop would.
func deltaPayloadScript() string {
	return "#!/bin/sh\n" +
		"cat \"$PWD/payload.txt\"\n" +
		"if [ -f \"$PWD/exitcode\" ]; then exit \"$(cat \"$PWD/exitcode\")\"; fi\n"
}

func deltaBasePayload() string {
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "payload line %d with deterministic filler words for hashing\n", i)
	}
	return b.String()
}

func newDeltaTestEngine(t *testing.T, cfg config.Config) (*engine.Engine, config.Paths, string, string) {
	t.Helper()
	script := testutil.WriteExecutable(t, t.TempDir(), "deltaraw", deltaPayloadScript())
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), deltaTestProfiles())
	cwd := t.TempDir()
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload())
	return e, paths, script, cwd
}

func runDeltaCommand(t *testing.T, e *engine.Engine, cfg config.Config, script string, cwd string) engine.Result {
	t.Helper()
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command:  []string{script},
		Display:  []string{"deltaraw"},
		Cwd:      cwd,
		Advanced: cfg.Advanced,
	}, false)
	if err != nil {
		t.Fatalf("execute deltaraw: %v", err)
	}
	return result
}

func TestDeltaRenderEmitsDigestForChangedRerun(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script, cwd := newDeltaTestEngine(t, cfg)

	first := runDeltaCommand(t, e, cfg, script, cwd)
	if first.DeltaRef != "" || first.DedupRef != "" || strings.Contains(first.Display, "since last run") {
		t.Fatalf("first run must render fully: %#v", first)
	}

	const addedLine = "brand new payload line added after the edit for delta checks"
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+addedLine+"\n")
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if len(second.DeltaRef) != dedup.RefLength || second.DedupRef != "" {
		t.Fatalf("expected delta digest with baseline ref, got %#v", second)
	}
	if !strings.HasPrefix(second.Display, "since last run (") || !strings.Contains(second.Display, ": +1 -0 lines") {
		t.Fatalf("expected delta header, got %q", second.Display)
	}
	if !strings.Contains(second.Display, "[baseline: szr expand "+second.DeltaRef+"]") {
		t.Fatalf("expected baseline expand hint, got %q", second.Display)
	}
	if !strings.Contains(second.Display, "\n+"+addedLine) {
		t.Fatalf("expected added line in digest, got %q", second.Display)
	}
	if history.EstimateTokens(second.Display) >= history.EstimateTokens(first.Display) {
		t.Fatalf("digest must be strictly cheaper: %q vs %q", second.Display, first.Display)
	}

	// The baseline ref expands to the first run's byte-exact raw output.
	store := dedup.New(paths.DataDir)
	entry, ok, err := store.FindRef(second.DeltaRef)
	if err != nil || !ok {
		t.Fatalf("find baseline ref: ok=%t err=%v", ok, err)
	}
	data, err := store.ReadArtifact(entry)
	if err != nil {
		t.Fatalf("read baseline artifact: %v", err)
	}
	if string(data) != first.RawCombined && string(data) != first.RawCombined+"\n" {
		t.Fatalf("baseline mismatch:\nartifact=%q\nraw=%q", data, first.RawCombined)
	}

	// A rerun identical to the delta run dedups against it: byte-identical
	// output always outranks another digest.
	third := runDeltaCommand(t, e, cfg, script, cwd)
	if third.DedupRef == "" || third.DeltaRef != "" || !strings.Contains(third.Display, "unchanged from previous run") {
		t.Fatalf("expected identical rerun to dedup, got %#v", third)
	}
}

func TestDeltaRenderLeadsWithNewlyFailingCriticalLines(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script, cwd := newDeltaTestEngine(t, cfg)
	basePayload := deltaBasePayload() + "--- FAIL: TestAlphaScenario (0.01s)\n"
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), basePayload)
	testutil.MustWriteFile(t, filepath.Join(cwd, "exitcode"), "2")

	first := runDeltaCommand(t, e, cfg, script, cwd)
	if first.ExitCode != 2 || first.DeltaRef != "" {
		t.Fatalf("unexpected first failing run: %#v", first)
	}

	const boringLine = "an unremarkable added detail line that carries no failure signal"
	const failLine = "--- FAIL: TestFreshRegression (0.01s)"
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), basePayload+boringLine+"\n"+failLine+"\n")
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.ExitCode != 2 || second.DeltaRef == "" {
		t.Fatalf("expected failing rerun to delta-render: %#v", second)
	}
	lines := strings.Split(second.Display, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "since last run (") {
		t.Fatalf("unexpected digest shape: %q", second.Display)
	}
	if lines[1] != "+"+failLine {
		t.Fatalf("expected newly-failing critical line to lead the digest, got %q", second.Display)
	}
	if !strings.Contains(second.Display, "+"+boringLine) {
		t.Fatalf("expected the boring added line in digest, got %q", second.Display)
	}
}

func TestDeltaRenderExitMismatchKeepsNormalRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script, cwd := newDeltaTestEngine(t, cfg)

	first := runDeltaCommand(t, e, cfg, script, cwd)
	if first.ExitCode != 0 {
		t.Fatalf("expected first run success: %#v", first)
	}

	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"a fresh diagnostic line the failing rerun introduces\n")
	testutil.MustWriteFile(t, filepath.Join(cwd, "exitcode"), "3")
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.ExitCode != 3 {
		t.Fatalf("expected failing rerun: %#v", second)
	}
	if second.DeltaRef != "" || strings.Contains(second.Display, "since last run") {
		t.Fatalf("expected no digest across exit codes: %#v", second)
	}
}

func TestDeltaRenderExpiredWindowKeepsNormalRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script, cwd := newDeltaTestEngine(t, cfg)

	runDeltaCommand(t, e, cfg, script, cwd)
	backdateDedupIndex(t, paths.DataDir, 2*time.Hour)

	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"a change that lands long after the window expired\n")
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.DeltaRef != "" || strings.Contains(second.Display, "since last run") {
		t.Fatalf("expected no digest outside the window: %#v", second)
	}
}

func TestDeltaRenderOversizedOutputBailsToNormalRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script, cwd := newDeltaTestEngine(t, cfg)
	var b strings.Builder
	for i := 1; i <= 20001; i++ {
		fmt.Fprintf(&b, "oversized payload line %d beyond the delta diff bound\n", i)
	}
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), b.String())

	runDeltaCommand(t, e, cfg, script, cwd)
	b.WriteString("one extra line after the edit\n")
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), b.String())
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.DeltaRef != "" || strings.Contains(second.Display, "since last run") {
		t.Fatalf("expected oversized rerun to keep the normal render: %#v", second)
	}
}

func TestDeltaRenderNotCheaperKeepsNormalRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script, cwd := newDeltaTestEngine(t, cfg)

	first := runDeltaCommand(t, e, cfg, script, cwd)
	var b strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "entirely rewritten output row %d sharing nothing with before\n", i)
	}
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), b.String())
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.DeltaRef != "" || strings.Contains(second.Display, "since last run") {
		t.Fatalf("expected full rewrite to keep the normal render: %#v", second)
	}
	if second.Display == first.Display {
		t.Fatalf("expected the fresh render, got the old one: %q", second.Display)
	}
}

func TestDeltaRenderDisabledKeepsNormalRenderAndDedup(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Advanced.DeltaRender = false
	e, _, script, cwd := newDeltaTestEngine(t, cfg)

	runDeltaCommand(t, e, cfg, script, cwd)
	testutil.MustWriteFile(t, filepath.Join(cwd, "payload.txt"), deltaBasePayload()+"a changed line the disabled delta feature must ignore\n")
	second := runDeltaCommand(t, e, cfg, script, cwd)
	if second.DeltaRef != "" || strings.Contains(second.Display, "since last run") {
		t.Fatalf("expected no digest with the flag off: %#v", second)
	}

	// Session dedup is unaffected by the delta flag.
	third := runDeltaCommand(t, e, cfg, script, cwd)
	if third.DedupRef == "" || !strings.Contains(third.Display, "unchanged from previous run") {
		t.Fatalf("expected dedup to keep working with delta off: %#v", third)
	}
}
