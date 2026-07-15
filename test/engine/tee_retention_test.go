package engine_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/internal/engine"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/internal/teeindex"
	"github.com/devr-tools/szr/test/testutil"
)

const teeDefaultFileCap = int64(config.DefaultTeeMaxFileMB) << 20

const teeBulkSummary = "bulk capture summary: payload streamed with head and tail markers intact\n" +
	"first payload marker retained for orientation and downstream expand checks\n" +
	"second payload marker retained for orientation and downstream expand checks\n" +
	"third payload marker retained for orientation and downstream expand checks\n" +
	"fourth payload marker retained for orientation and downstream expand checks\n" +
	"closing summary line kept only to make the render big enough to reference"

const teeFailSummary = "capture run failed while streaming the oversized payload\n" +
	"--- FAIL: TestPayloadScenario (0.01s)\n" +
	"assertion mismatch reported at payload_case.go:42 during the streaming pass\n" +
	"remaining suites passed without any diagnostics worth keeping around\n" +
	"closing summary line kept only to make the render big enough to reference"

func teeTestProfiles() []engine.Profile {
	return []engine.Profile{
		{
			Name: "bigbulk",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "bigbulk"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return teeBulkSummary
			},
		},
		{
			Name: "bigfail",
			Match: func(inv engine.Invocation) bool {
				return len(inv.Display) > 0 && inv.Display[0] == "bigfail"
			},
			Render: func(engine.Invocation, engine.Execution) string {
				return teeFailSummary
			},
		},
	}
}

func newTeeTestEngine(t *testing.T, cfg config.Config) (*engine.Engine, config.Paths) {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	e := engine.New(cfg, paths, history.New(paths.HistoryFile), teeTestProfiles())
	return e, paths
}

func runTeeCommand(t *testing.T, e *engine.Engine, cfg config.Config, script string, display string, cwd string) engine.Result {
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

// buildTeePayload emits a stream larger than the default 4 MiB cap with a
// distinctive first and last line, and a middle line that lands strictly
// inside the region capping drops (past the 2 MiB head, before the 2 MiB
// tail).
func buildTeePayload(middle string) []byte {
	var b bytes.Buffer
	b.WriteString("tee head marker line\n")
	line := strings.Repeat("x", 63) + "\n"
	for i := 0; i < 49152; i++ { // 3 MiB
		b.WriteString(line)
	}
	b.WriteString(middle)
	b.WriteString("\n")
	for i := 0; i < 49152; i++ { // 3 MiB
		b.WriteString(line)
	}
	b.WriteString("tee tail marker line\n")
	return b.Bytes()
}

func writeTeePayloadDir(t *testing.T, middle string) string {
	t.Helper()
	dir := t.TempDir()
	writeTeePayload(t, dir, middle)
	return dir
}

func writeTeePayload(t *testing.T, dir string, middle string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "payload.txt"), buildTeePayload(middle), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}
}

func TestCapTeeContentKeepsSmallStreams(t *testing.T) {
	t.Parallel()
	raw := []byte("short capture output")
	if got := engine.CapTeeContent(raw, 64); !bytes.Equal(got, raw) {
		t.Fatalf("small stream must pass through, got %q", got)
	}
}

func TestCapTeeContentKeepsHeadAndTailWithMarker(t *testing.T) {
	t.Parallel()
	raw := []byte(strings.Repeat("a", 100) + strings.Repeat("b", 100) + strings.Repeat("c", 100))
	capped := engine.CapTeeContent(raw, 64)

	if !bytes.HasPrefix(capped, []byte(strings.Repeat("a", 32))) {
		t.Fatalf("expected 32-byte head, got %q", capped[:40])
	}
	if !bytes.HasSuffix(capped, []byte(strings.Repeat("c", 32))) {
		t.Fatalf("expected 32-byte tail, got %q", capped[len(capped)-40:])
	}
	want := fmt.Sprintf("[szr tee truncated: %d bytes omitted of %d total", len(raw)-64, len(raw))
	if !strings.Contains(string(capped), want) {
		t.Fatalf("expected marker %q in %q", want, capped)
	}
	if !strings.Contains(string(capped), "full stream sha256 ") {
		t.Fatalf("expected full-stream hash in marker: %q", capped)
	}
}

// TestCapTeeContentDiscriminatesMiddleChanges is the dedup-correctness
// contract: two streams that agree on head, tail, and length but differ in
// the dropped middle must still produce different capped files, because the
// marker embeds the full-stream hash.
func TestCapTeeContentDiscriminatesMiddleChanges(t *testing.T) {
	t.Parallel()
	build := func(middle byte) []byte {
		raw := bytes.Repeat([]byte{'p'}, 300)
		raw[150] = middle
		return raw
	}
	first := engine.CapTeeContent(build('1'), 64)
	repeat := engine.CapTeeContent(build('1'), 64)
	changed := engine.CapTeeContent(build('2'), 64)

	if !bytes.Equal(first, repeat) {
		t.Fatal("identical streams must produce identical capped files")
	}
	if bytes.Equal(first, changed) {
		t.Fatal("middle-differing streams must not produce identical capped files")
	}
}

func TestTeeCaptureCapsFailureArtifact(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _ := newTeeTestEngine(t, cfg)
	cwd := writeTeePayloadDir(t, "middle variant alpha")
	script := testutil.WriteExecutable(t, t.TempDir(), "bigfail", "#!/bin/sh\ncat payload.txt\nexit 3\n")

	result := runTeeCommand(t, e, cfg, script, "bigfail", cwd)
	if result.ExitCode != 3 || result.TeePath == "" {
		t.Fatalf("expected failure artifact, got %#v", result)
	}

	info, err := os.Stat(result.TeePath)
	if err != nil {
		t.Fatalf("stat artifact: %v", err)
	}
	if info.Size() > teeDefaultFileCap+256 {
		t.Fatalf("artifact not capped: %d bytes", info.Size())
	}

	data := testutil.MustReadFile(t, result.TeePath)
	payloadLen := len(buildTeePayload("middle variant alpha"))
	marker := fmt.Sprintf("[szr tee truncated: %d bytes omitted of %d total", int64(payloadLen)-teeDefaultFileCap, payloadLen)
	content := string(data)
	if !strings.Contains(content, "tee head marker line") {
		t.Fatal("capped artifact lost the head of the stream")
	}
	if !strings.Contains(content, "tee tail marker line") {
		t.Fatal("capped artifact lost the tail of the stream")
	}
	if !strings.Contains(content, marker) {
		t.Fatalf("expected marker %q in artifact", marker)
	}
	if strings.Contains(content, "middle variant alpha") {
		t.Fatal("middle of the stream should have been dropped")
	}
}

// TestSessionDedupSurvivesTeeCapping proves capping the capture file does not
// weaken dedup: identical oversized runs still dedup, and a run whose output
// changed only inside the dropped middle region must not.
func TestSessionDedupSurvivesTeeCapping(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, _ := newTeeTestEngine(t, cfg)
	cwd := writeTeePayloadDir(t, "middle variant alpha")
	script := testutil.WriteExecutable(t, t.TempDir(), "bigbulk", "#!/bin/sh\ncat payload.txt\n")

	first := runTeeCommand(t, e, cfg, script, "bigbulk", cwd)
	if first.DedupRef != "" {
		t.Fatalf("first run must not dedup: %#v", first.DedupRef)
	}
	second := runTeeCommand(t, e, cfg, script, "bigbulk", cwd)
	if len(second.DedupRef) != dedup.RefLength {
		t.Fatalf("identical oversized rerun must dedup, got ref %q", second.DedupRef)
	}

	// Same length, same head, same tail; only the dropped middle changes.
	writeTeePayload(t, cwd, "middle variant bravo")
	third := runTeeCommand(t, e, cfg, script, "bigbulk", cwd)
	if third.DedupRef != "" {
		t.Fatalf("middle-changed rerun must not dedup, got ref %q", third.DedupRef)
	}
	if strings.Contains(third.Display, "unchanged from previous run") {
		t.Fatalf("middle-changed rerun rendered a dedup reference: %q", third.Display)
	}
}

func seedTeeArtifacts(t *testing.T, teeDir string, count int, size int64) []string {
	t.Helper()
	store := teeindex.New(teeDir)
	base := time.Now().Add(-24 * time.Hour)
	paths := make([]string, 0, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(teeDir, fmt.Sprintf("%d_fake%04d.log", base.UnixNano()+int64(i), i))
		if err := os.WriteFile(path, []byte("seed artifact\n"), 0o644); err != nil {
			t.Fatalf("seed artifact: %v", err)
		}
		if size > 0 {
			if err := os.Truncate(path, size); err != nil {
				t.Fatalf("truncate artifact: %v", err)
			}
		}
		stamp := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
		if err := store.Append(teeindex.Entry{Timestamp: stamp, Path: path, Command: "seed", ExitCode: 1}); err != nil {
			t.Fatalf("append index entry: %v", err)
		}
		paths = append(paths, path)
	}
	return paths
}

func countTeeLogs(t *testing.T, teeDir string) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(teeDir, "*.log"))
	if err != nil {
		t.Fatalf("glob tee dir: %v", err)
	}
	return len(matches)
}

func TestTeePruneBoundsDirectoryFileCount(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths := newTeeTestEngine(t, cfg)
	seeded := seedTeeArtifacts(t, paths.TeeDir, config.DefaultTeeMaxDirFiles+5, 0)

	stalePartial := filepath.Join(paths.TeeDir, "szr-tee-stale.partial")
	freshPartial := filepath.Join(paths.TeeDir, "szr-tee-fresh.partial")
	testutil.MustWriteFile(t, stalePartial, "stale")
	testutil.MustWriteFile(t, freshPartial, "fresh")
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stalePartial, old, old); err != nil {
		t.Fatalf("chtimes partial: %v", err)
	}

	script := testutil.WriteExecutable(t, t.TempDir(), "bigfail", "#!/bin/sh\necho failing run detail output\nexit 3\n")
	result := runTeeCommand(t, e, cfg, script, "bigfail", t.TempDir())
	if result.TeePath == "" {
		t.Fatalf("expected failure artifact, got %#v", result)
	}

	if got := countTeeLogs(t, paths.TeeDir); got != config.DefaultTeeMaxDirFiles {
		t.Fatalf("expected %d artifacts after prune, got %d", config.DefaultTeeMaxDirFiles, got)
	}
	for _, path := range seeded[:6] {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected oldest artifact %s to be pruned", path)
		}
	}
	if _, err := os.Stat(result.TeePath); err != nil {
		t.Fatalf("newest artifact must survive pruning: %v", err)
	}
	if _, err := os.Stat(stalePartial); !os.IsNotExist(err) {
		t.Fatal("stale partial capture should be removed")
	}
	if _, err := os.Stat(freshPartial); err != nil {
		t.Fatalf("fresh partial capture must survive: %v", err)
	}

	entries, err := teeindex.New(paths.TeeDir).LoadAll()
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	pruned := make(map[string]struct{})
	for _, path := range seeded[:6] {
		pruned[path] = struct{}{}
	}
	for _, entry := range entries {
		if _, gone := pruned[entry.Path]; gone {
			t.Fatalf("index still references pruned artifact %s", entry.Path)
		}
	}
	if len(entries) != config.DefaultTeeMaxDirFiles {
		t.Fatalf("expected %d index entries, got %d", config.DefaultTeeMaxDirFiles, len(entries))
	}
}

func TestTeePruneBoundsDirectorySize(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	e, paths := newTeeTestEngine(t, cfg)
	// Sparse files: apparent size drives pruning without writing 300 MiB.
	seeded := seedTeeArtifacts(t, paths.TeeDir, 3, 100<<20)

	script := testutil.WriteExecutable(t, t.TempDir(), "bigfail", "#!/bin/sh\necho failing run detail output\nexit 3\n")
	result := runTeeCommand(t, e, cfg, script, "bigfail", t.TempDir())
	if result.TeePath == "" {
		t.Fatalf("expected failure artifact, got %#v", result)
	}

	if _, err := os.Stat(seeded[0]); !os.IsNotExist(err) {
		t.Fatal("expected oldest oversized artifact to be pruned")
	}
	for _, path := range seeded[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("artifact %s should survive size pruning: %v", path, err)
		}
	}
	if _, err := os.Stat(result.TeePath); err != nil {
		t.Fatalf("newest artifact must survive pruning: %v", err)
	}
}
