package diagnostics_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/diagnostics"
)

func TestStoreAppendsAndReadsEventsWithOwnerOnlyPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	store := diagnostics.New(path)
	event := diagnostics.Event{Version: diagnostics.SchemaVersion, Type: diagnostics.EventRunFinal, RunID: "run-1", Timestamp: time.Now(), Profile: "go-test-json"}
	if err := store.Append(event); err != nil {
		t.Fatalf("append: %v", err)
	}
	events, err := store.ReadAll()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(events) != 1 || events[0].RunID != event.RunID || events[0].Profile != event.Profile {
		t.Fatalf("unexpected events: %#v", events)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("event file mode = %o, want 600", info.Mode().Perm())
	}
}

func TestProviderUsageAggregateIsCounterOnlyAndClampsInvalidValues(t *testing.T) {
	event := diagnostics.ProviderUsageAggregate("gateway-correlation", -2, 100, 140, 50, -1, 8)
	if event.Version != diagnostics.SchemaVersion || event.Type != diagnostics.EventProviderUsageAggregate {
		t.Fatalf("unexpected schema: %#v", event)
	}
	if event.GatewayCorrelationID != "gateway-correlation" || event.AggregateRunCount != 0 {
		t.Fatalf("unexpected aggregate correlation/count: %#v", event)
	}
	if event.RawTokensEst != 100 || event.EmittedTokensEst != 140 || event.SavedTokensEst != 0 {
		t.Fatalf("unexpected szr counters: %#v", event)
	}
	if event.ProviderInputTokens != 50 || event.ProviderCacheTokens != 0 || event.ProviderOutputTokens != 8 {
		t.Fatalf("unexpected provider counters: %#v", event)
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"command", "transcript", "prompt", "session_id", "path"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("aggregate unexpectedly contains %q: %s", forbidden, encoded)
		}
	}
}
