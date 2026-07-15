package cli_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/devr-tools/szr/internal/cli"
	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
	"github.com/devr-tools/szr/test/testutil"
)

func spreadCostApp(t *testing.T, cfg config.Config, records []history.Record) *cli.App {
	t.Helper()
	paths := testutil.Paths(t.TempDir())
	testutil.EnsurePaths(t, paths)
	store := history.New(paths.HistoryFile)
	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("append history record: %v", err)
		}
	}
	return cli.NewWithDependencies("test", cfg, paths, store, testutil.AppEngine(t, paths))
}

func spreadCostRecords() []history.Record {
	return []history.Record{
		{
			Timestamp:         time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
			Command:           "szr go test ./...",
			Profile:           "go-test",
			ProfileConfidence: "high",
			DurationMS:        40,
			RawTokens:         500_000,
			FilteredTokens:    100_000,
			SavedTokens:       400_000,
			SavingsPct:        80,
		},
	}
}

func TestSpreadCostSectionRendersWithDefaultRate(t *testing.T) {
	app := spreadCostApp(t, config.Default(), spreadCostRecords())

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--cost")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread --cost output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	for _, want := range []string{
		"estimated cost:",
		"estimated cost avoided: $1.20 at $3.00/Mtok input",
		"raw input $1.50 -> emitted output $0.30",
		"≈ 2.0× a 200k-token context",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected cost section line %q, got %q", want, stdout)
		}
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread output stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if strings.Contains(stdout, "estimated cost") {
		t.Fatalf("expected no cost section without --cost, got %q", stdout)
	}
}

func TestSpreadCostRateResolutionPrecedence(t *testing.T) {
	cfg := config.Default()
	cfg.CostRatePerMtok = 5.0
	app := spreadCostApp(t, cfg, spreadCostRecords())

	code, stdout, _ := testutil.RunApp(t, app, "spread", "--cost")
	if code != 0 || !strings.Contains(stdout, "estimated cost avoided: $2.00 at $5.00/Mtok input") {
		t.Fatalf("expected config rate to beat default, got %q (code=%d)", stdout, code)
	}

	code, stdout, _ = testutil.RunApp(t, app, "spread", "--cost", "--rate=2.50")
	if code != 0 || !strings.Contains(stdout, "estimated cost avoided: $1.00 at $2.50/Mtok input") {
		t.Fatalf("expected --rate flag to beat config rate, got %q (code=%d)", stdout, code)
	}

	code, stdout, _ = testutil.RunApp(t, app, "spread", "--cost", "--rate", "4")
	if code != 0 || !strings.Contains(stdout, "at $4.00/Mtok input") {
		t.Fatalf("expected separate-value --rate form, got %q (code=%d)", stdout, code)
	}

	code, stdout, _ = testutil.RunApp(t, app, "spread", "--rate=2")
	if code != 0 || !strings.Contains(stdout, "at $2.00/Mtok input") {
		t.Fatalf("expected --rate to imply the cost section, got %q (code=%d)", stdout, code)
	}

	zeroApp := spreadCostApp(t, config.Config{}, spreadCostRecords())
	code, stdout, _ = testutil.RunApp(t, zeroApp, "spread", "--cost")
	if code != 0 || !strings.Contains(stdout, "at $3.00/Mtok input") {
		t.Fatalf("expected built-in default rate fallback, got %q (code=%d)", stdout, code)
	}
}

func TestSpreadCostJSONShape(t *testing.T) {
	app := spreadCostApp(t, config.Default(), spreadCostRecords())

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--cost", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread --cost --json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload struct {
		history.Summary
		Cost *struct {
			RatePerMtokUSD      float64 `json:"rate_per_mtok_usd"`
			CostAvoidedUSD      float64 `json:"estimated_cost_avoided_usd"`
			RawCostUSD          float64 `json:"raw_cost_usd"`
			OutputCostUSD       float64 `json:"output_cost_usd"`
			ContextWindowTokens int     `json:"context_window_tokens"`
			ContextWindowsSaved float64 `json:"context_windows_saved"`
		} `json:"cost"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal spread cost json: %v", err)
	}
	if payload.Commands != 1 || payload.SavedTokens != 400_000 {
		t.Fatalf("expected summary fields alongside cost, got %#v", payload.Summary)
	}
	if payload.Cost == nil {
		t.Fatalf("expected cost section in json payload, got %q", stdout)
	}
	cost := payload.Cost
	if cost.RatePerMtokUSD != 3.0 || !approxEquals(cost.CostAvoidedUSD, 1.2) {
		t.Fatalf("unexpected cost rate/avoided values: %#v", cost)
	}
	if !approxEquals(cost.RawCostUSD, 1.5) || !approxEquals(cost.OutputCostUSD, 0.3) {
		t.Fatalf("unexpected raw/output cost values: %#v", cost)
	}
	if cost.ContextWindowTokens != 200_000 || !approxEquals(cost.ContextWindowsSaved, 2.0) {
		t.Fatalf("unexpected context window values: %#v", cost)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread --json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var plain map[string]any
	if err := json.Unmarshal([]byte(stdout), &plain); err != nil {
		t.Fatalf("unmarshal spread json: %v", err)
	}
	if _, ok := plain["cost"]; ok {
		t.Fatalf("expected no cost key without --cost, got %q", stdout)
	}
}

func TestSpreadCostZeroHistory(t *testing.T) {
	app := spreadCostApp(t, config.Default(), nil)

	code, stdout, stderr := testutil.RunApp(t, app, "spread", "--cost")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread --cost stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	if !strings.Contains(stdout, "no tracked commands yet") {
		t.Fatalf("expected zero-history notice, got %q", stdout)
	}

	code, stdout, stderr = testutil.RunApp(t, app, "spread", "--cost", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("unexpected spread --cost --json stdout=%q stderr=%q code=%d", stdout, stderr, code)
	}
	var payload struct {
		Cost *struct {
			RatePerMtokUSD float64 `json:"rate_per_mtok_usd"`
			CostAvoidedUSD float64 `json:"estimated_cost_avoided_usd"`
		} `json:"cost"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal zero-history cost json: %v", err)
	}
	if payload.Cost == nil || payload.Cost.RatePerMtokUSD != 3.0 || payload.Cost.CostAvoidedUSD != 0 {
		t.Fatalf("expected zeroed cost section with rate, got %q", stdout)
	}
}

func TestSpreadCostRejectsInvalidRate(t *testing.T) {
	app := spreadCostApp(t, config.Default(), spreadCostRecords())
	for _, args := range [][]string{
		{"spread", "--rate=abc"},
		{"spread", "--rate=-1"},
		{"spread", "--rate=0"},
		{"spread", "--rate"},
	} {
		code, _, stderr := testutil.RunApp(t, app, args...)
		if code != 2 || !strings.Contains(stderr, "--rate") {
			t.Fatalf("expected rate rejection for %v, got code=%d stderr=%q", args, code, stderr)
		}
	}
}

func approxEquals(got, want float64) bool {
	diff := got - want
	return diff > -0.0001 && diff < 0.0001
}
