package dedup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/dedup"
)

func TestAppendLoadAndLookup(t *testing.T) {
	t.Parallel()
	store := dedup.New(t.TempDir())
	older := entryFixture("aaaa1111", time.Now().Add(-10*time.Minute))
	newer := entryFixture("bbbb2222", time.Now())
	for _, entry := range []dedup.Entry{older, newer} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	entries, err := store.LoadAll()
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected load: %#v err=%v", entries, err)
	}

	latest, ok, err := store.Latest()
	if err != nil || !ok || latest.RawHash != newer.RawHash {
		t.Fatalf("unexpected latest: %#v ok=%t err=%v", latest, ok, err)
	}

	found, ok, err := store.FindRef("aaaa")
	if err != nil || !ok || found.RawHash != older.RawHash {
		t.Fatalf("unexpected prefix lookup: %#v ok=%t err=%v", found, ok, err)
	}

	if _, ok, _ := store.FindRef("aa"); ok {
		t.Fatal("expected too-short ref to miss")
	}
	if _, ok, _ := store.FindRef("ffff9999"); ok {
		t.Fatal("expected unknown ref to miss")
	}
}

func TestMatchesRespectsKeyAndWindow(t *testing.T) {
	t.Parallel()
	store := dedup.New(t.TempDir())
	now := time.Now()
	match := entryFixture("cccc3333", now.Add(-5*time.Minute))
	expired := entryFixture("cccc3333", now.Add(-2*time.Hour))
	otherCwd := entryFixture("cccc3333", now)
	otherCwd.Cwd = "/elsewhere"
	otherExit := entryFixture("cccc3333", now)
	otherExit.ExitCode = 1
	otherHash := entryFixture("dddd4444", now)
	for _, entry := range []dedup.Entry{match, expired, otherCwd, otherExit, otherHash} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	matches, err := store.Matches(dedup.Key{
		CommandFingerprint: match.CommandFingerprint,
		Cwd:                match.Cwd,
		ExitCode:           match.ExitCode,
		RawHash:            match.RawHash,
	}, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("matches: %v", err)
	}
	if len(matches) != 1 || !matches[0].Timestamp.Equal(match.Timestamp) {
		t.Fatalf("unexpected matches: %#v", matches)
	}
}

func TestMatchesIsolatesScopes(t *testing.T) {
	t.Parallel()
	store := dedup.New(t.TempDir())
	now := time.Now()
	machine := entryFixture("aaaa7777", now)
	scoped := entryFixture("aaaa7777", now)
	scoped.Scope = "swarm-a"
	otherScope := entryFixture("aaaa7777", now)
	otherScope.Scope = "swarm-b"
	for _, entry := range []dedup.Entry{machine, scoped, otherScope} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	since := now.Add(-30 * time.Minute)
	for _, tc := range []struct {
		scope string
		want  string
	}{
		{scope: "", want: ""},
		{scope: "swarm-a", want: "swarm-a"},
		{scope: "swarm-b", want: "swarm-b"},
	} {
		matches, err := store.Matches(dedup.Key{
			CommandFingerprint: machine.CommandFingerprint,
			Cwd:                machine.Cwd,
			ExitCode:           machine.ExitCode,
			RawHash:            machine.RawHash,
			Scope:              tc.scope,
		}, since)
		if err != nil {
			t.Fatalf("matches scope %q: %v", tc.scope, err)
		}
		if len(matches) != 1 || matches[0].Scope != tc.want {
			t.Fatalf("expected exactly the %q-scope entry, got %#v", tc.scope, matches)
		}
	}
}

func TestCommandMatchesScopeWindowAndOrder(t *testing.T) {
	t.Parallel()
	store := dedup.New(t.TempDir())
	now := time.Now()
	older := entryFixture("bbbb8888", now.Add(-10*time.Minute))
	newer := entryFixture("cccc9999", now)
	newer.ExitCode = 1
	expired := entryFixture("dddd0000", now.Add(-2*time.Hour))
	scoped := entryFixture("eeee1111", now)
	scoped.Scope = "swarm-a"
	otherCwd := entryFixture("ffff2222", now)
	otherCwd.Cwd = "/elsewhere"
	for _, entry := range []*dedup.Entry{&newer, &expired, &scoped, &otherCwd} {
		entry.CommandFingerprint = older.CommandFingerprint
	}
	for _, entry := range []dedup.Entry{older, newer, expired, scoped, otherCwd} {
		if err := store.Append(entry); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	matches, err := store.CommandMatches(older.CommandFingerprint, older.Cwd, "", now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("command matches: %v", err)
	}
	if len(matches) != 2 || matches[0].RawHash != newer.RawHash || matches[1].RawHash != older.RawHash {
		t.Fatalf("expected newest-first machine-scope matches across exit codes, got %#v", matches)
	}

	scopedMatches, err := store.CommandMatches(scoped.CommandFingerprint, scoped.Cwd, "swarm-a", now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("scoped command matches: %v", err)
	}
	if len(scopedMatches) != 1 || scopedMatches[0].Scope != "swarm-a" {
		t.Fatalf("expected only the swarm-a entry, got %#v", scopedMatches)
	}
}

func TestLoadAllToleratesCorruptLines(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	store := dedup.New(dataDir)
	if err := store.Append(entryFixture("eeee5555", time.Now())); err != nil {
		t.Fatalf("append: %v", err)
	}
	indexPath := filepath.Join(dataDir, dedup.DirName, "index.jsonl")
	file, err := os.OpenFile(indexPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	if _, err := file.WriteString("{\"raw_hash\": torn-line"); err != nil {
		t.Fatalf("write torn line: %v", err)
	}
	_ = file.Close()

	entries, err := store.LoadAll()
	if err != nil || len(entries) != 1 || entries[0].RawHash != rawHashFor("eeee5555") {
		t.Fatalf("expected torn line to be skipped: %#v err=%v", entries, err)
	}
}

func TestArtifactRoundTripAndVerify(t *testing.T) {
	t.Parallel()
	store := dedup.New(t.TempDir())
	payload := []byte("raw output éè\n binary: \x00\x01\x02\n")
	path, hash, err := store.WriteArtifact(payload)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	entry := entryFixture("ffff6666", time.Now())
	entry.ArtifactPath = path
	entry.ArtifactHash = hash

	if !store.VerifyArtifact(entry) {
		t.Fatal("expected artifact to verify")
	}
	data, err := store.ReadArtifact(entry)
	if err != nil || string(data) != string(payload) {
		t.Fatalf("unexpected artifact read: %q err=%v", data, err)
	}

	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}
	if store.VerifyArtifact(entry) {
		t.Fatal("expected tampered artifact to fail verification")
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove artifact: %v", err)
	}
	if store.VerifyArtifact(entry) {
		t.Fatal("expected missing artifact to fail verification")
	}
}

func TestCompactionDropsOldEntriesAndOrphanedArtifacts(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	store := dedup.New(dataDir)

	oldPath, oldHash, err := store.WriteArtifact([]byte("old artifact"))
	if err != nil {
		t.Fatalf("write old artifact: %v", err)
	}
	oldest := entryFixture("0000aaaa", time.Now().Add(-30*time.Hour))
	oldest.ArtifactPath = oldPath
	oldest.ArtifactHash = oldHash
	oldest.Command = strings.Repeat("x", 4096)
	if err := store.Append(oldest); err != nil {
		t.Fatalf("append oldest: %v", err)
	}

	livePath, liveHash, err := store.WriteArtifact([]byte("live artifact"))
	if err != nil {
		t.Fatalf("write live artifact: %v", err)
	}
	// Enough oversized entries to push the index past its size cap so the
	// append-time compaction fires and drops the oldest entry.
	for i := 0; i < 300; i++ {
		entry := entryFixture("1111bbbb", time.Now().Add(-time.Duration(300-i)*time.Minute))
		entry.ArtifactPath = livePath
		entry.ArtifactHash = liveHash
		entry.Command = strings.Repeat("y", 4096)
		if err := store.Append(entry); err != nil {
			t.Fatalf("append filler %d: %v", i, err)
		}
	}

	entries, err := store.LoadAll()
	if err != nil {
		t.Fatalf("load after compaction: %v", err)
	}
	for _, entry := range entries {
		if entry.RawHash == rawHashFor("0000aaaa") {
			t.Fatal("expected oldest entry to be compacted away")
		}
	}
	if _, statErr := os.Stat(oldPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected orphaned artifact to be removed, got %v", statErr)
	}
	if _, statErr := os.Stat(livePath); statErr != nil {
		t.Fatalf("expected live artifact to survive: %v", statErr)
	}
}

func TestEntryRef(t *testing.T) {
	t.Parallel()
	entry := dedup.Entry{RawHash: "0123456789abcdef0123"}
	if entry.Ref() != "0123456789ab" {
		t.Fatalf("unexpected ref: %q", entry.Ref())
	}
	short := dedup.Entry{RawHash: "0123"}
	if short.Ref() != "0123" {
		t.Fatalf("unexpected short ref: %q", short.Ref())
	}
}

func entryFixture(hashSeed string, at time.Time) dedup.Entry {
	return dedup.Entry{
		Timestamp:          at,
		RawHash:            rawHashFor(hashSeed),
		ArtifactHash:       rawHashFor(hashSeed),
		ArtifactPath:       "/nonexistent/" + hashSeed,
		Command:            "git status",
		CommandFingerprint: "fp-" + hashSeed[:4],
		Cwd:                "/repo",
		ExitCode:           0,
		RawBytes:           128,
	}
}

// rawHashFor builds a deterministic 64-char pseudo hash from a seed so
// prefix lookups behave like real SHA-256 hex strings.
func rawHashFor(seed string) string {
	return (seed + strings.Repeat("0", 64))[:64]
}
