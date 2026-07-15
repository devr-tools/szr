package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
)

// spreadContextWindowTokens anchors savings in a familiar vendor-neutral
// unit: one 200k-token context window.
const spreadContextWindowTokens = 200_000

type spreadJSONPayload struct {
	history.Summary
	Cost *spreadCostReport `json:"cost,omitempty"`
}

type spreadCostReport struct {
	RatePerMtokUSD      float64 `json:"rate_per_mtok_usd"`
	CostAvoidedUSD      float64 `json:"estimated_cost_avoided_usd"`
	RawCostUSD          float64 `json:"raw_cost_usd"`
	OutputCostUSD       float64 `json:"output_cost_usd"`
	ContextWindowTokens int     `json:"context_window_tokens"`
	ContextWindowsSaved float64 `json:"context_windows_saved"`
}

func (o *spreadOptions) applyRate(value string) bool {
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil || rate <= 0 {
		fmt.Fprintf(os.Stderr, "szr: invalid --rate %q (want USD per million input tokens, e.g. 3.00)\n", value)
		return false
	}
	o.rate = rate
	o.cost = true
	return true
}

func (a *App) spreadCostReport(summary history.Summary, opts spreadOptions) *spreadCostReport {
	if !opts.cost {
		return nil
	}
	rate := resolveSpreadCostRate(opts.rate, a.config.CostRatePerMtok)
	return &spreadCostReport{
		RatePerMtokUSD:      rate,
		CostAvoidedUSD:      tokensToUSD(summary.SavedTokens, rate),
		RawCostUSD:          tokensToUSD(summary.RawTokens, rate),
		OutputCostUSD:       tokensToUSD(summary.FilteredTokens, rate),
		ContextWindowTokens: spreadContextWindowTokens,
		ContextWindowsSaved: float64(summary.SavedTokens) / spreadContextWindowTokens,
	}
}

func resolveSpreadCostRate(flagRate, configRate float64) float64 {
	if flagRate > 0 {
		return flagRate
	}
	if configRate > 0 {
		return configRate
	}
	return config.DefaultCostRatePerMtok
}

func tokensToUSD(tokens int, ratePerMtok float64) float64 {
	return float64(tokens) / 1_000_000 * ratePerMtok
}

func renderSpreadCost(ui spreadUI, cost *spreadCostReport) {
	if cost == nil {
		return
	}
	ui.section("estimated cost:")
	fmt.Printf("  estimated cost avoided: $%.2f at $%.2f/Mtok input\n", cost.CostAvoidedUSD, cost.RatePerMtokUSD)
	fmt.Printf("  raw input $%.2f -> emitted output $%.2f\n", cost.RawCostUSD, cost.OutputCostUSD)
	fmt.Printf("  ≈ %.1f× a 200k-token context\n", cost.ContextWindowsSaved)
}
