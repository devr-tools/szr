package cli_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/dedup"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func newExpandApp(t *testing.T) (*cli.App, *dedup.Store) {
	t.Helper()
	root := t.TempDir()
	paths := testutil.Paths(root)
	testutil.EnsurePaths(t, paths)
	app := cli.NewWithDependencies("test", config.Default(), paths, history.New(paths.HistoryFile), testutil.AppEngine(t, paths))
	return app, dedup.New(paths.DataDir)
}

func seedExpandEntry(t *testing.T, store *dedup.Store, hashSeed string, payload []byte, at time.Time) dedup.Entry {
	t.Helper()
	path, artifactHash, err := store.WriteArtifact(payload)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	entry := dedup.Entry{
		Timestamp:          at,
		RawHash:            (hashSeed + strings.Repeat("0", 64))[:64],
		ArtifactHash:       artifactHash,
		ArtifactPath:       path,
		Command:            "git status",
		CommandFingerprint: "fp",
		Cwd:                "/repo",
		RawBytes:           int64(len(payload)),
	}
	if err := store.Append(entry); err != nil {
		t.Fatalf("append entry: %v", err)
	}
	return entry
}

func TestExpandRoundTripsUnicodePayloadByteExact(t *testing.T) {
	app, store := newExpandApp(t)
	payload := []byte("naïve → résumé ✓\ntab\tseparated\nfinal line without newline")
	entry := seedExpandEntry(t, store, "aaaa1111", payload, time.Now())

	code, stdout, stderr := testutil.RunApp(t, app, "expand", entry.Ref())
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected expand result stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if stdout != string(payload) {
		t.Fatalf("expected byte-exact expansion, got %q want %q", stdout, payload)
	}
}

func TestExpandRoundTripsLargePayloadByteExact(t *testing.T) {
	app, store := newExpandApp(t)
	line := "large payload line with deterministic content ü§ 0123456789\n"
	payload := []byte(strings.Repeat(line, 12000))
	entry := seedExpandEntry(t, store, "bbbb2222", payload, time.Now())

	code, stdout, stderr := testutil.RunApp(t, app, "expand", entry.Ref())
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected expand result stderr=%q code=%d", stderr, code)
	}
	if stdout != string(payload) {
		t.Fatalf("expected byte-exact large expansion, got %d bytes want %d", len(stdout), len(payload))
	}
}

func TestExpandLastPrintsMostRecentEntry(t *testing.T) {
	app, store := newExpandApp(t)
	seedExpandEntry(t, store, "cccc3333", []byte("older payload\n"), time.Now().Add(-time.Minute))
	seedExpandEntry(t, store, "dddd4444", []byte("newest payload\n"), time.Now())

	code, stdout, stderr := testutil.RunApp(t, app, "expand", "--last")
	if code != 0 || stderr != "" || stdout != "newest payload\n" {
		t.Fatalf("unexpected expand --last stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestExpandUnknownRefFails(t *testing.T) {
	app, _ := newExpandApp(t)
	code, stdout, stderr := testutil.RunApp(t, app, "expand", "deadbeef1234")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "unknown or expired ref") {
		t.Fatalf("unexpected unknown-ref result stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestExpandLastWithoutEntriesFails(t *testing.T) {
	app, _ := newExpandApp(t)
	code, stdout, stderr := testutil.RunApp(t, app, "expand", "--last")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "no dedup references recorded yet") {
		t.Fatalf("unexpected empty --last result stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
}

func TestExpandArgumentValidation(t *testing.T) {
	app, _ := newExpandApp(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no args", []string{"expand"}, "expand requires a reference"},
		{"short ref", []string{"expand", "ab"}, "too short"},
		{"unknown flag", []string{"expand", "--bad"}, "unknown expand flag"},
		{"two refs", []string{"expand", "aaaa1111", "bbbb2222"}, "at most one reference"},
	}
	for _, tc := range cases {
		code, stdout, stderr := testutil.RunApp(t, app, tc.args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%s: unexpected result stdout=%q stderr=%q code=%d", tc.name, stdout, stderr, code)
		}
	}
}

func TestExpandTruncatedEntryNotesTruncationOnStderr(t *testing.T) {
	app, store := newExpandApp(t)
	payload := []byte("stored prefix of a very large raw output\n")
	entry := seedExpandEntry(t, store, "eeee5555", payload, time.Now().Add(-time.Minute))
	entry.Timestamp = time.Now()
	entry.Truncated = true
	entry.RawBytes = 5 << 20
	if err := store.Append(entry); err != nil {
		t.Fatalf("append truncated entry: %v", err)
	}

	code, stdout, stderr := testutil.RunApp(t, app, "expand", entry.Ref())
	if code != 0 || stdout != string(payload) {
		t.Fatalf("unexpected truncated expand stdout=%q code=%d", stdout, code)
	}
	if !strings.Contains(stderr, "truncated") {
		t.Fatalf("expected truncation note on stderr, got %q", stderr)
	}
}

// TestExpandEndToEndThroughRun drives the real flow: the same command run
// twice through the CLI emits a reference the second time, and expand
// recovers the raw output byte-exact.
func TestExpandEndToEndThroughRun(t *testing.T) {
	binDir := t.TempDir()
	script := testutil.WriteExecutable(t, binDir, "emit", "#!/bin/sh\n"+
		"i=1\nwhile [ $i -le 12 ]; do\n"+
		"  echo \"payload line $i naïve ✓ with deterministic filler words\"\n"+
		"  i=$((i + 1))\ndone\n")
	workDir := t.TempDir()
	restore := testutil.Chdir(t, workDir)
	defer restore()

	app := testutil.NewTestApp(t)
	code, first, _ := testutil.RunApp(t, app, "run", script)
	if code != 0 {
		t.Fatalf("first run failed: code=%d output=%q", code, first)
	}
	if strings.Contains(first, "unchanged from previous run") {
		t.Fatalf("first run must not reference: %q", first)
	}

	code, second, _ := testutil.RunApp(t, app, "run", script)
	if code != 0 || !strings.Contains(second, "unchanged from previous run") {
		t.Fatalf("expected second run to reference: code=%d output=%q", code, second)
	}
	refPattern := regexp.MustCompile(`\[ref: ([0-9a-f]{12}) - expand with: szr expand ([0-9a-f]{12})\]`)
	match := refPattern.FindStringSubmatch(second)
	if match == nil || match[1] != match[2] {
		t.Fatalf("expected expand hint in reference render, got %q", second)
	}

	code, expanded, stderr := testutil.RunApp(t, app, "expand", match[1])
	if code != 0 || stderr != "" {
		t.Fatalf("expand failed: code=%d stderr=%q", code, stderr)
	}
	var expected strings.Builder
	for i := 1; i <= 12; i++ {
		expected.WriteString("payload line ")
		expected.WriteString(strconv.Itoa(i))
		expected.WriteString(" naïve ✓ with deterministic filler words\n")
	}
	if expanded != expected.String() {
		t.Fatalf("expected byte-exact recovery:\n got %q\nwant %q", expanded, expected.String())
	}

	code, last, _ := testutil.RunApp(t, app, "expand", "--last")
	if code != 0 || last != expanded {
		t.Fatalf("expected --last to match the ref expansion, got %q", last)
	}
}
