package budgethints

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOutcomeStoreRollsBackHarmfulHint(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	hint := Hint{Version: CurrentVersion, Profile: "go-test", Direction: DirectionTighten, Samples: 20, ExpiresAt: now.Add(time.Hour)}
	store := NewOutcomeStore(filepath.Join(t.TempDir(), "outcomes.jsonl"))
	for i := 0; i < 5; i++ {
		if err := store.Append(Outcome{At: now, Profile: hint.Profile, ExpiresAt: hint.ExpiresAt, Fallback: i == 0}); err != nil {
			t.Fatal(err)
		}
	}
	if !store.ShouldRollback(hint, now) {
		t.Fatal("expected rollback after harmful outcome rate")
	}
}
