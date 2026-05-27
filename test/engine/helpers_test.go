package engine_test

import (
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/engine"
)

func TestCombineStreams(t *testing.T) {
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
}

func TestSanitizeFileName(t *testing.T) {
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
}

func TestResolveBudget(t *testing.T) {
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

	pytestBudget := engine.ResolveBudget(engine.Profile{Name: "pytest", Budget: engine.OutputBudget{MaxLines: 8}, Confidence: engine.ConfidenceHigh}, engine.Invocation{}, 12)
	if pytestBudget.MaxLines < 15 {
		t.Fatalf("expected pytest budget expansion, got %#v", pytestBudget)
	}

	pythonToolingBudget := engine.ResolveBudget(engine.Profile{Name: "python-tooling", Budget: engine.OutputBudget{MaxLines: 8}, Confidence: engine.ConfidenceHigh}, engine.Invocation{}, 12)
	if pythonToolingBudget.MaxLines < 12 {
		t.Fatalf("expected python-tooling budget expansion, got %#v", pythonToolingBudget)
	}

	jsWorkspaceBudget := engine.ResolveBudget(
		engine.Profile{Name: "js-workspace", Budget: engine.OutputBudget{MaxLines: 12}, Confidence: engine.ConfidenceHigh},
		engine.Classify(engine.Invocation{Command: []string{"npm", "run", "lint"}}),
		12,
	)
	if jsWorkspaceBudget.MaxLines >= 12 {
		t.Fatalf("expected js-workspace lint budget tightening, got %#v", jsWorkspaceBudget)
	}

	tscBudget := engine.ResolveBudget(
		engine.Profile{Name: "js-workspace", Budget: engine.OutputBudget{MaxLines: 8}, Confidence: engine.ConfidenceHigh},
		engine.Classify(engine.Invocation{Command: []string{"tsc", "--noEmit"}}),
		12,
	)
	if tscBudget.MaxLines < 14 {
		t.Fatalf("expected tsc budget expansion, got %#v", tscBudget)
	}
}

func TestResolveBudgetAgentMode(t *testing.T) {
	standardMediumBudget := engine.ResolveBudget(
		engine.Profile{Budget: engine.OutputBudget{MaxLines: 12}, Confidence: engine.ConfidenceMedium},
		engine.Invocation{},
		12,
	)
	agentBudget := engine.ResolveBudget(
		engine.Profile{Budget: engine.OutputBudget{MaxLines: 12}, Confidence: engine.ConfidenceMedium},
		engine.Invocation{ReasoningBudgetMode: "agent"},
		12,
	)
	if agentBudget.MaxLines >= standardMediumBudget.MaxLines || agentBudget.MinFailures != 1 || agentBudget.MinAnchors != 1 || agentBudget.MinHints != 1 {
		t.Fatalf("expected reasoning-budget agent mode to tighten and add contracts, got %#v", agentBudget)
	}

	aggressiveBudget := engine.ResolveBudget(
		engine.Profile{Budget: engine.OutputBudget{MaxLines: 12}, Confidence: engine.ConfidenceMedium},
		engine.Invocation{ReasoningBudgetMode: "aggressive"},
		12,
	)
	if aggressiveBudget.MaxLines >= agentBudget.MaxLines || aggressiveBudget.MinFailures != 1 || aggressiveBudget.MinAnchors != 1 || aggressiveBudget.MinHints != 1 {
		t.Fatalf("expected aggressive reasoning budget to tighten beyond agent mode, got %#v", aggressiveBudget)
	}
}

func TestDecideFastPath(t *testing.T) {
	fast := engine.DecideFastPath(engine.Profile{}, engine.Invocation{Command: []string{"echo", "ok"}}, 64, 12, 2*time.Millisecond, 0)
	if !fast.BypassCompression || fast.Reason == "" {
		t.Fatalf("expected tiny output fast path, got %#v", fast)
	}

	fast = engine.DecideFastPath(engine.Profile{LatencyBudget: time.Millisecond}, engine.Invocation{Command: []string{"echo", "ok"}}, 500, 100, 2*time.Millisecond, 0)
	if fast.BypassCompression || !fast.WarnLatency {
		t.Fatalf("expected latency warning without bypass, got %#v", fast)
	}

	fast = engine.DecideFastPath(engine.Profile{}, engine.Invocation{Command: []string{"echo", "ok"}}, 64, 12, 2*time.Millisecond, 1)
	if fast.BypassCompression {
		t.Fatalf("did not expect failure bypass, got %#v", fast)
	}
}
