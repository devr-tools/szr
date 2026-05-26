package engine

import (
	"fmt"
	"strings"

	"github.com/devr-tools/szr/internal/history"
)

const (
	compressionContractMinRawTokens = 40
	compressionContractMinTokens    = 8
	compressionContractRetainedNum  = 1
	compressionContractRetainedDen  = 5
)

func enforceCompressionContract(text string, rawCombined string, budget OutputBudget, plan RecoveryPlan, passthrough bool) (string, RecoveryPlan, bool) {
	if passthrough {
		return text, plan, false
	}
	rawTokens := history.EstimateTokens(rawCombined)
	if rawTokens < compressionContractMinRawTokens {
		return text, plan, false
	}
	allowedTokens := compressionContractAllowedTokens(rawTokens, budget)
	filteredTokens := history.EstimateTokens(text)
	if allowedTokens <= 0 || filteredTokens <= allowedTokens {
		return text, plan, false
	}
	compressed := hardCapTokens(text, allowedTokens)
	if strings.TrimSpace(compressed) == "" {
		return text, plan, false
	}
	updatedPlan := plan
	updatedPlan.Kind = RecoveryKindFullOutput
	updatedPlan.RequireRawCapture = strings.TrimSpace(rawCombined) != ""
	updatedPlan.Summary = compressionRecoverySummary(plan.Summary, allowedTokens, rawTokens)
	return compressed, updatedPlan, true
}

func compressionContractAllowedTokens(rawTokens int, budget OutputBudget) int {
	if rawTokens <= 0 {
		return 0
	}
	allowed := scaleIntCeil(rawTokens, compressionContractRetainedNum, compressionContractRetainedDen)
	if allowed < compressionContractMinTokens {
		allowed = compressionContractMinTokens
	}
	if budget.MaxTokens > 0 && budget.MaxTokens < allowed {
		allowed = budget.MaxTokens
	}
	if allowed < 1 {
		return 1
	}
	return allowed
}

func hardCapTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	fields := strings.Fields(text)
	if len(fields) <= maxTokens {
		return strings.Join(fields, " ")
	}
	if maxTokens == 1 {
		return "..."
	}
	for keep := maxTokens - 1; keep > 0; keep-- {
		kept := append([]string{}, fields[:keep]...)
		kept = append(kept, "...")
		candidate := strings.Join(kept, " ")
		if history.EstimateTokens(candidate) <= maxTokens {
			return candidate
		}
	}
	return "..."
}

func compressionRecoverySummary(existing string, allowedTokens int, rawTokens int) string {
	summary := fmt.Sprintf("compressed to %d tokens from %d", allowedTokens, rawTokens)
	if strings.TrimSpace(existing) == "" {
		return summary
	}
	return existing + "; " + summary
}
