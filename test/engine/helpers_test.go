package engine_test

import (
	"strings"
	"testing"
	"time"

	"szr/internal/engine"
)

func TestHelpers(t *testing.T) {
	cases := []struct {
		stdout string
		stderr string
		want   string
	}{
		{"", "", ""},
		{"out", "", "out"},
		{"", "err", "err"},
		{"out", "err", "out\nerr"},
	}
	for _, tc := range cases {
		if got := engine.CombineStreams(tc.stdout, tc.stderr); got != tc.want {
			t.Fatalf("combine streams mismatch: got %q want %q", got, tc.want)
		}
	}

	if got := engine.SanitizeFileName("***"); got != "output" {
		t.Fatalf("unexpected empty sanitize fallback: %q", got)
	}
	if got := engine.SanitizeFileName("abc-123"); got != "abc_123" {
		t.Fatalf("unexpected sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName("AbC9"); got != "AbC9" {
		t.Fatalf("unexpected uppercase sanitize result: %q", got)
	}
	if got := engine.SanitizeFileName(strings.Repeat("x", 60)); len(got) != 48 {
		t.Fatalf("expected truncated sanitize result, got %d chars", len(got))
	}

	budget := engine.ResolveBudget(engine.Profile{Budget: engine.OutputBudget{MaxLines: 5}}, engine.Invocation{}, 12)
	if budget.MaxLines != 5 || budget.MaxBytes != 800 {
		t.Fatalf("unexpected resolved budget: %#v", budget)
	}

	budget = engine.ResolveBudget(engine.Profile{}, engine.Invocation{}, 12)
	if budget.MaxLines != 12 || budget.MaxBytes != 1920 {
		t.Fatalf("unexpected default budget: %#v", budget)
	}

	ultraBudget := engine.ResolveBudget(engine.Profile{Budget: engine.OutputBudget{MaxLines: 10}, Confidence: engine.ConfidenceHigh}, engine.Invocation{UltraCompact: true}, 12)
	if ultraBudget.MaxLines >= 10 {
		t.Fatalf("expected ultra-compact budget shrink, got %#v", ultraBudget)
	}

	verboseBudget := engine.ResolveBudget(engine.Profile{Budget: engine.OutputBudget{MaxLines: 10}, Confidence: engine.ConfidenceHigh}, engine.Invocation{Verbose: 2}, 12)
	if verboseBudget.MaxLines <= 10 {
		t.Fatalf("expected verbose budget expansion, got %#v", verboseBudget)
	}

	lowConfidenceBudget := engine.ResolveBudget(engine.Profile{Budget: engine.OutputBudget{MaxLines: 10}, Confidence: engine.ConfidenceLow}, engine.Invocation{}, 12)
	if lowConfidenceBudget.MaxLines <= 10 {
		t.Fatalf("expected low-confidence budget expansion, got %#v", lowConfidenceBudget)
	}

	fast := engine.DecideFastPath(engine.Profile{}, 64, 12, 2*time.Millisecond, 0)
	if !fast.BypassCompression || fast.Reason == "" {
		t.Fatalf("expected tiny output fast path, got %#v", fast)
	}

	fast = engine.DecideFastPath(engine.Profile{LatencyBudget: time.Millisecond}, 500, 100, 2*time.Millisecond, 0)
	if fast.BypassCompression || !fast.WarnLatency {
		t.Fatalf("expected latency warning without bypass, got %#v", fast)
	}

	fast = engine.DecideFastPath(engine.Profile{}, 64, 12, 2*time.Millisecond, 1)
	if fast.BypassCompression {
		t.Fatalf("did not expect failure bypass, got %#v", fast)
	}
}
