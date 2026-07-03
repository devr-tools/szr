package engine_test

import (
	"context"
	"encoding/json"
	"os"
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

const dedupBulkSummary = "bulk summary: 12 payload lines with alpha beta gamma delta markers\n" +
	"first payload marker alpha retained for orientation and downstream checks\n" +
	"second payload marker beta retained for orientation and downstream checks\n" +
	"third payload marker gamma retained for orientation and downstream checks\n" +
	"fourth payload marker delta retained for orientation and downstream checks\n" +
	"fifth payload marker epsilon retained for orientation and downstream checks"

const dedupFailSummary = "test run failed with two suites reporting assertion problems\n" +
	"--- FAIL: TestAlphaScenario (0.01s)\n" +
	"--- FAIL: TestBetaScenario (0.02s)\n" +
	"assertion mismatch reported at scenario_case.go:42 during the beta pass\n" +
	"remaining suites passed without any diagnostics worth keeping around\n" +
	"closing summary line kept only to make the render big enough to reference"

func dedupTestProfiles() []engine.Profile {
	return []engine.Profile{
		{
			Name: "bulk",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "bulk"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return dedupBulkSummary
			},
		},
		{
			Name: "failbulk",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "failbulk"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return dedupFailSummary
			},
		},
		{
			Name: "tinybulk",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "tinybulk"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return "ok done"
			},
		},
	}
}

// dedupBulkScript emits enough deterministic output for a reference-worthy
// render while staying under the compression contract's arming threshold, so
// the render pipeline adds no artifact suffixes that would make assertions
// environment-dependent.
func dedupBulkScript() string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("i=1\nwhile [ $i -le 12 ]; do\n")
	b.WriteString("  echo \"payload line $i with deterministic filler words for hashing\"\n")
	b.WriteString("  i=$((i + 1))\ndone\n")
	return b.String()
}

func newDedupTestEngine(t *testing.T, cfg config.Config) (*engine.Engine, config.Paths, string) {
	t.Helper()
	binDir := t.TempDir()
	script := testutil.WriteExecutable(t, binDir, "bulk", dedupBulkScript())
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), dedupTestProfiles())
	return e, paths, script
}

func runDedupCommand(t *testing.T, e *engine.Engine, cfg config.Config, script string, display string, cwd string) engine.Result {
	t.Helper()
	result, err := e.Execute(context.Background(), engine.Invocation{
		Command:  []string{script},
		Display:  []string{display},
		Cwd:      cwd,
		Advanced: cfg.Advanced,
	}, false)
	if err != nil {
		t.Fatalf("execute %s: %v", display, err)
	}
	return result
}

func TestSessionDedupIdenticalRerunEmitsReference(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	first := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if first.DedupRef != "" || strings.Contains(first.Display, "unchanged from previous run") {
		t.Fatalf("first run must render fully: %#v", first)
	}
	if first.Display != dedupBulkSummary {
		t.Fatalf("unexpected first render: %q", first.Display)
	}

	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if len(second.DedupRef) != dedup.RefLength {
		t.Fatalf("expected %d-char dedup ref, got %#v", dedup.RefLength, second)
	}
	wantRefLine := "unchanged from previous run ("
	if !strings.Contains(second.Display, wantRefLine) || !strings.Contains(second.Display, "x2 identical") {
		t.Fatalf("expected reference render, got %q", second.Display)
	}
	if !strings.Contains(second.Display, "[ref: "+second.DedupRef+" - expand with: szr expand "+second.DedupRef+"]") {
		t.Fatalf("expected expand hint with ref, got %q", second.Display)
	}
	firstLine := strings.SplitN(dedupBulkSummary, "\n", 2)[0]
	if !strings.HasPrefix(second.Display, firstLine+"\n") {
		t.Fatalf("expected orientation first line %q, got %q", firstLine, second.Display)
	}
	if history.EstimateTokens(second.Display) >= history.EstimateTokens(first.Display) {
		t.Fatalf("reference render must be cheaper: %q vs %q", second.Display, first.Display)
	}

	third := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if third.DedupRef != second.DedupRef || !strings.Contains(third.Display, "x3 identical") {
		t.Fatalf("expected x3 identical with same ref, got %#v", third)
	}

	// The stored artifact must round-trip the raw output byte-exact.
	store := dedup.New(paths.DataDir)
	entry, ok, err := store.FindRef(second.DedupRef)
	if err != nil || !ok {
		t.Fatalf("find ref: ok=%t err=%v", ok, err)
	}
	data, err := store.ReadArtifact(entry)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(data) != first.RawCombined+"\n" && string(data) != first.RawCombined {
		t.Fatalf("artifact mismatch:\nartifact=%q\nraw=%q", data, first.RawCombined)
	}
}

func TestSessionDedupDifferentCwdDoesNotFire(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script := newDedupTestEngine(t, cfg)

	first := runDedupCommand(t, e, cfg, script, "bulk", t.TempDir())
	second := runDedupCommand(t, e, cfg, script, "bulk", t.TempDir())
	if first.DedupRef != "" || second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup across directories: %#v", second)
	}
}

func TestSessionDedupDifferentExitCodeDoesNotFire(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	binDir := t.TempDir()
	script := testutil.WriteExecutable(t, binDir, "bulk", dedupBulkScript()+"if [ -f \"$PWD/make-it-fail\" ]; then exit 3; fi\n")
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), dedupTestProfiles())
	cwd := t.TempDir()

	first := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if first.ExitCode != 0 {
		t.Fatalf("expected first run success: %#v", first)
	}
	testutil.MustWriteFile(t, filepath.Join(cwd, "make-it-fail"), "x")
	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if second.ExitCode != 3 {
		t.Fatalf("expected failing rerun: %#v", second)
	}
	if second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup across exit codes: %#v", second)
	}
}

func TestSessionDedupExpiredWindowDoesNotFire(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	backdateDedupIndex(t, paths.DataDir, 2*time.Hour)

	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup outside the window: %#v", second)
	}
}

func TestSessionDedupFailingRerunKeepsFailureLines(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	binDir := t.TempDir()
	script := testutil.WriteExecutable(t, binDir, "failbulk", dedupBulkScript()+"exit 2\n")
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), dedupTestProfiles())
	cwd := t.TempDir()

	first := runDedupCommand(t, e, cfg, script, "failbulk", cwd)
	if first.ExitCode != 2 || first.DedupRef != "" {
		t.Fatalf("unexpected first failing run: %#v", first)
	}
	second := runDedupCommand(t, e, cfg, script, "failbulk", cwd)
	if second.DedupRef == "" || !strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected failing rerun to dedup: %#v", second)
	}
	refIdx := strings.Index(second.Display, "unchanged from previous run")
	for _, failLine := range []string{"--- FAIL: TestAlphaScenario", "--- FAIL: TestBetaScenario"} {
		failIdx := strings.Index(second.Display, failLine)
		if failIdx < 0 || failIdx > refIdx {
			t.Fatalf("expected %q above the reference, got %q", failLine, second.Display)
		}
	}
}

func TestSessionDedupMissingArtifactFallsBackToFullRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if second.DedupRef == "" {
		t.Fatalf("expected second run to dedup: %#v", second)
	}
	removeDedupArtifacts(t, paths.DataDir)

	third := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if third.DedupRef != "" || strings.Contains(third.Display, "unchanged from previous run") {
		t.Fatalf("expected full render after artifact loss: %#v", third)
	}
	if third.Display != dedupBulkSummary {
		t.Fatalf("expected the normal render, got %q", third.Display)
	}

	// The fallback run stores a fresh artifact, so dedup recovers afterwards.
	fourth := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if fourth.DedupRef == "" || !strings.Contains(fourth.Display, "unchanged from previous run") {
		t.Fatalf("expected dedup to self-heal, got %#v", fourth)
	}
}

func TestSessionDedupCorruptArtifactFallsBackToFullRender(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	corruptDedupArtifacts(t, paths.DataDir)

	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected full render for corrupt artifact: %#v", second)
	}
}

func TestSessionDedupDisabledNeverFires(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Advanced.SessionDedup = false
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	second := runDedupCommand(t, e, cfg, script, "bulk", cwd)
	if second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected no dedup with the flag off: %#v", second)
	}
	if _, err := os.Stat(filepath.Join(paths.DataDir, dedup.DirName)); !os.IsNotExist(err) {
		t.Fatalf("expected no dedup state on disk, got %v", err)
	}
}

func TestSessionDedupUltraCompactNeverFires(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	for i := 0; i < 2; i++ {
		result, err := e.Execute(context.Background(), engine.Invocation{
			Command:      []string{script},
			Display:      []string{"bulk"},
			Cwd:          cwd,
			UltraCompact: true,
			Advanced:     cfg.Advanced,
		}, false)
		if err != nil {
			t.Fatalf("execute ultra-compact: %v", err)
		}
		if result.DedupRef != "" || strings.Contains(result.Display, "unchanged from previous run") {
			t.Fatalf("expected no dedup in ultra-compact mode: %#v", result)
		}
	}
}

func TestSessionDedupPassthroughNeverFires(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	for i := 0; i < 2; i++ {
		result, err := e.Execute(context.Background(), engine.Invocation{
			Command:  []string{script},
			Display:  []string{"bulk"},
			Cwd:      cwd,
			Advanced: cfg.Advanced,
		}, true)
		if err != nil {
			t.Fatalf("execute passthrough: %v", err)
		}
		if result.DedupRef != "" || strings.Contains(result.Display, "unchanged from previous run") {
			t.Fatalf("expected no dedup in passthrough mode: %#v", result)
		}
	}
}

func TestSessionDedupTinyRenderSkipped(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	binDir := t.TempDir()
	script := testutil.WriteExecutable(t, binDir, "tinybulk", dedupBulkScript())
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), dedupTestProfiles())
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "tinybulk", cwd)
	second := runDedupCommand(t, e, cfg, script, "tinybulk", cwd)
	if second.DedupRef != "" || strings.Contains(second.Display, "unchanged from previous run") {
		t.Fatalf("expected tiny render to skip dedup: %#v", second)
	}
}

// TestSessionDedupCaptureFilesAreCleanedUp guards against leaking the raw
// capture files dedup retains on successful exits.
func TestSessionDedupCaptureFilesAreCleanedUp(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths, script := newDedupTestEngine(t, cfg)
	cwd := t.TempDir()

	runDedupCommand(t, e, cfg, script, "bulk", cwd)
	runDedupCommand(t, e, cfg, script, "bulk", cwd)

	entries, err := os.ReadDir(paths.TeeDir)
	if err != nil {
		t.Fatalf("read tee dir: %v", err)
	}
	for _, entry := range entries {
		t.Fatalf("expected no leftover capture files, found %s", entry.Name())
	}
}

func backdateDedupIndex(t *testing.T, dataDir string, by time.Duration) {
	t.Helper()
	store := dedup.New(dataDir)
	entries, err := store.LoadAll()
	if err != nil || len(entries) == 0 {
		t.Fatalf("load dedup index: %#v err=%v", entries, err)
	}
	indexPath := filepath.Join(dataDir, dedup.DirName, "index.jsonl")
	file, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("rewrite dedup index: %v", err)
	}
	enc := json.NewEncoder(file)
	for _, entry := range entries {
		entry.Timestamp = entry.Timestamp.Add(-by)
		if err := enc.Encode(entry); err != nil {
			t.Fatalf("encode backdated entry: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close dedup index: %v", err)
	}
}

func removeDedupArtifacts(t *testing.T, dataDir string) {
	t.Helper()
	for _, path := range dedupArtifactPaths(t, dataDir) {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove artifact %s: %v", path, err)
		}
	}
}

func corruptDedupArtifacts(t *testing.T, dataDir string) {
	t.Helper()
	for _, path := range dedupArtifactPaths(t, dataDir) {
		if err := os.WriteFile(path, []byte("corrupted bytes"), 0o644); err != nil {
			t.Fatalf("corrupt artifact %s: %v", path, err)
		}
	}
}

func dedupArtifactPaths(t *testing.T, dataDir string) []string {
	t.Helper()
	pattern := filepath.Join(dataDir, dedup.DirName, "artifact-*.raw")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob artifacts: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("expected dedup artifacts under %s", pattern)
	}
	return paths
}
