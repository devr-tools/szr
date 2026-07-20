package budgethints

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLookupPrefersExactUnexpiredHint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hints.json")
	store := New(path)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	if err := store.Replace([]Hint{
		{Version: CurrentVersion, Profile: "go-test", Direction: DirectionTighten, Samples: 40, ExpiresAt: now.Add(time.Hour), Suggested: Target{MaxLines: 8}},
		{Version: CurrentVersion, Profile: "go-test", Fingerprint: "exact", Direction: DirectionLoosen, Samples: 40, ExpiresAt: now.Add(2 * time.Hour), Suggested: Target{MaxLines: 30}},
		{Version: CurrentVersion, Profile: "go-test", Fingerprint: "old", Direction: DirectionLoosen, Samples: 40, ExpiresAt: now.Add(-time.Hour), Suggested: Target{MaxLines: 30}},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	hint, err := store.Lookup("go-test", "exact", now)
	if err != nil || hint == nil {
		t.Fatalf("lookup: hint=%#v err=%v", hint, err)
	}
	if hint.Fingerprint != "exact" || hint.Direction != DirectionLoosen {
		t.Fatalf("expected exact hint, got %#v", hint)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected owner-only hint file, got %o", info.Mode().Perm())
	}
}

func TestReplaceRejectsUnsafeHint(t *testing.T) {
	store := New(filepath.Join(t.TempDir(), "hints.json"))
	err := store.Replace([]Hint{{Version: CurrentVersion, Profile: "x", Direction: DirectionTighten, Samples: 1}})
	if err == nil {
		t.Fatal("expected missing expiry to be rejected")
	}
}
