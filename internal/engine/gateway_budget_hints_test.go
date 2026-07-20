package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/budgethints"
)

func TestGatewayBudgetHintAdapterAppliesCappedAdjustment(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	store := budgethints.New(filepath.Join(t.TempDir(), "hints.json"))
	if err := store.Replace([]budgethints.Hint{{
		Version: budgethints.CurrentVersion, Profile: "go-test", Fingerprint: "", Direction: budgethints.DirectionLoosen,
		Samples: 40, ExpiresAt: now.Add(time.Hour), Suggested: budgethints.Target{MaxLines: 100, MaxBytes: 16000, MaxTokens: 3200},
	}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	adapter := NewGatewayBudgetHintAdapterWithOptions(store, GatewayBudgetHintAdapterOptions{Now: func() time.Time { return now }})
	budget, adaptation := adapter.AdaptBudget(Profile{Name: "go-test"}, Invocation{Display: []string{"go", "test"}}, OutputBudget{MaxLines: 20, MaxBytes: 3200, MaxTokens: 640})
	if adaptation == nil || adaptation.Source != BudgetAdaptationSourceGateway {
		t.Fatalf("expected gateway adaptation, got %#v", adaptation)
	}
	// Local guardrails win over an oversized gateway request: 15%% loosening.
	if budget.MaxLines != 23 || budget.MaxBytes != 3680 || budget.MaxTokens != 736 {
		t.Fatalf("unexpected capped budget: %#v", budget)
	}
}

func TestGatewayBudgetHintAdapterRejectsExpiredAndLowSampleHints(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	for name, hint := range map[string]budgethints.Hint{
		"expired":     {Version: budgethints.CurrentVersion, Profile: "go-test", Direction: budgethints.DirectionTighten, Samples: 40, ExpiresAt: now.Add(-time.Second), Suggested: budgethints.Target{MaxLines: 1}},
		"low-samples": {Version: budgethints.CurrentVersion, Profile: "go-test", Direction: budgethints.DirectionTighten, Samples: 19, ExpiresAt: now.Add(time.Hour), Suggested: budgethints.Target{MaxLines: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			store := budgethints.New(filepath.Join(t.TempDir(), "hints.json"))
			if err := store.Replace([]budgethints.Hint{hint}); err != nil {
				t.Fatalf("replace: %v", err)
			}
			adapter := NewGatewayBudgetHintAdapterWithOptions(store, GatewayBudgetHintAdapterOptions{Now: func() time.Time { return now }})
			base := OutputBudget{MaxLines: 20, MaxBytes: 3200, MaxTokens: 640}
			got, adaptation := adapter.AdaptBudget(Profile{Name: "go-test"}, Invocation{Display: []string{"go", "test"}}, base)
			if adaptation != nil || got != base {
				t.Fatalf("expected safe no-op, got budget=%#v adaptation=%#v", got, adaptation)
			}
		})
	}
}

func TestGatewayBudgetHintAdapterFailsClosedOnMalformedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hints.json")
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	adapter := NewGatewayBudgetHintAdapter(budgethints.New(path))
	base := OutputBudget{MaxLines: 20, MaxBytes: 3200, MaxTokens: 640}
	got, adaptation := adapter.AdaptBudget(Profile{Name: "go-test"}, Invocation{}, base)
	if adaptation != nil || got != base {
		t.Fatalf("expected malformed store no-op, got budget=%#v adaptation=%#v", got, adaptation)
	}
}
